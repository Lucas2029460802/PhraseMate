#import "native_darwin.h"

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <QuartzCore/QuartzCore.h>

#include <stdlib.h>
#include <string.h>

extern void goPMMessage(int id, const char *name, const char *argsJSON);
extern void goPMTray(int action);

enum {
	PM_TRAY_OPEN = 1,
	PM_TRAY_FLOAT = 2,
	PM_TRAY_SHORTCUT = 3,
	PM_TRAY_EXIT = 4
};

@interface PMKeyWindow : NSWindow
@end

@implementation PMKeyWindow
- (BOOL)canBecomeKeyWindow {
	return YES;
}
- (BOOL)canBecomeMainWindow {
	return YES;
}
@end

@interface PMMsgProxy : NSObject <WKScriptMessageHandler>
@property int windowID;
@end

@implementation PMMsgProxy
- (void)userContentController:(WKUserContentController *)userContentController
	  didReceiveScriptMessage:(WKScriptMessage *)message {
	NSString *name = @"";
	NSArray *args = @[];
	id body = message.body;
	if ([body isKindOfClass:[NSDictionary class]]) {
		NSDictionary *dict = body;
		id n = dict[@"name"];
		if ([n isKindOfClass:[NSString class]]) {
			name = n;
		}
		id a = dict[@"args"];
		if ([a isKindOfClass:[NSArray class]]) {
			args = a;
		}
	} else if ([body isKindOfClass:[NSString class]]) {
		name = body;
	}
	NSError *err = nil;
	NSData *data = [NSJSONSerialization dataWithJSONObject:args options:0 error:&err];
	if (!data) {
		data = [@"[]" dataUsingEncoding:NSUTF8StringEncoding];
	}
	NSString *json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
	goPMMessage(self.windowID, name.UTF8String, json.UTF8String);
}
@end

@interface PMWindowCtl : NSObject <NSWindowDelegate, WKNavigationDelegate>
@property int windowID;
@property int flags;
@property(strong) NSWindow *window;
@property(strong) WKWebView *webView;
@property(strong) PMMsgProxy *proxy;
@property(copy) NSString *bindJS;
- (void)applyBindScript;
@end

@implementation PMWindowCtl
- (BOOL)windowShouldClose:(NSWindow *)sender {
	if (self.flags & PM_WINDOW_HIDE_ON_CLOSE) {
		[sender orderOut:nil];
		return NO;
	}
	return YES;
}
- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation {
	[self applyBindScript];
}
- (void)applyBindScript {
	if (self.bindJS.length == 0) {
		return;
	}
	[self.webView evaluateJavaScript:self.bindJS completionHandler:nil];
}
@end

@interface PMAppDelegate : NSObject <NSApplicationDelegate>
@end

@implementation PMAppDelegate
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender {
	return NO;
}
- (BOOL)applicationSupportsSecureRestorableState:(NSApplication *)app {
	return NO;
}
- (void)applicationDidFinishLaunching:(NSNotification *)notification {
	[NSApp activateIgnoringOtherApps:YES];
}
@end

@interface PMTrayTarget : NSObject
- (void)onMenu:(NSMenuItem *)sender;
@end

@implementation PMTrayTarget
- (void)onMenu:(NSMenuItem *)sender {
	goPMTray((int)sender.tag);
}
@end

static PMAppDelegate *gAppDelegate;
static NSMutableDictionary<NSNumber *, PMWindowCtl *> *gWindows;
static int gNextID = 1;
static NSStatusItem *gStatusItem;
static PMTrayTarget *gTrayTarget;

static NSImage *PMImageFromRGBA(const uint8_t *rgba, int width, int height) {
	if (!rgba || width <= 0 || height <= 0) {
		return nil;
	}
	NSBitmapImageRep *rep = [[NSBitmapImageRep alloc]
	    initWithBitmapDataPlanes:NULL
		      pixelsWide:width
		      pixelsHigh:height
		   bitsPerSample:8
		 samplesPerPixel:4
			hasAlpha:YES
			isPlanar:NO
		  colorSpaceName:NSCalibratedRGBColorSpace
		    bitmapFormat:NSBitmapFormatAlphaNonpremultiplied
		     bytesPerRow:width * 4
		    bitsPerPixel:32];
	if (!rep || !rep.bitmapData) {
		return nil;
	}
	memcpy(rep.bitmapData, rgba, (size_t)width * (size_t)height * 4);
	NSImage *img = [[NSImage alloc] initWithSize:NSMakeSize(width, height)];
	[img addRepresentation:rep];
	return img;
}

static void PMOnMain(void (^block)(void)) {
	if ([NSThread isMainThread]) {
		block();
	} else {
		dispatch_sync(dispatch_get_main_queue(), block);
	}
}

static PMWindowCtl *PMGet(int id) {
	return gWindows[@(id)];
}

void PMInit(void) {
	PMOnMain(^{
	  if (![NSThread isMainThread]) {
		  return;
	  }
	  [NSApplication sharedApplication];
	  if (!gAppDelegate) {
		  gAppDelegate = [PMAppDelegate new];
	  }
	  [NSApp setDelegate:gAppDelegate];
	  [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
	  if (!gWindows) {
		  gWindows = [NSMutableDictionary new];
	  }
	});
}

void PMRun(void) {
	[NSApp run];
}

void PMQuit(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
	  [NSApp stop:nil];
	  NSEvent *ev = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
					   location:NSZeroPoint
				      modifierFlags:0
					  timestamp:0
				       windowNumber:0
					    context:nil
					    subtype:0
					      data1:0
					      data2:0];
	  [NSApp postEvent:ev atStart:YES];
	});
}

void PMSetAppIconRGBA(const uint8_t *rgba, int width, int height) {
	NSImage *img = PMImageFromRGBA(rgba, width, height);
	if (!img) {
		return;
	}
	PMOnMain(^{
	  [NSApp setApplicationIconImage:img];
	});
}

int PMWindowCreate(const char *title, int width, int height, int flags) {
	__block int outID = 0;
	NSString *nsTitle = title ? [NSString stringWithUTF8String:title] : @"PhraseMate";
	if (width < 80) {
		width = 80;
	}
	if (height < 40) {
		height = 40;
	}
	PMOnMain(^{
	  if (!gWindows) {
		  gWindows = [NSMutableDictionary new];
	  }
	  int wid = gNextID++;
	  NSRect rect = NSMakeRect(0, 0, width, height);
	  NSWindow *window = nil;
	  BOOL isFloat = (flags & PM_WINDOW_FLOAT) != 0;
	  if (isFloat) {
		  window = [[PMKeyWindow alloc] initWithContentRect:rect
							  styleMask:NSWindowStyleMaskBorderless
							    backing:NSBackingStoreBuffered
							      defer:NO];
		  window.level = NSFloatingWindowLevel;
		  window.hidesOnDeactivate = NO;
		  window.opaque = NO;
		  window.backgroundColor = [NSColor clearColor];
		  window.hasShadow = YES;
		  window.movableByWindowBackground = YES;
		  window.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces |
					     NSWindowCollectionBehaviorFullScreenAuxiliary |
					     NSWindowCollectionBehaviorIgnoresCycle;
		  window.titleVisibility = NSWindowTitleHidden;
	  } else {
		  NSUInteger style = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
				     NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable;
		  window = [[NSWindow alloc] initWithContentRect:rect
						      styleMask:style
							backing:NSBackingStoreBuffered
							  defer:NO];
		  window.title = nsTitle;
		  if (flags & PM_WINDOW_CENTER) {
			  [window center];
		  }
	  }
	  window.releasedWhenClosed = NO;
	  window.acceptsMouseMovedEvents = YES;

	  WKWebViewConfiguration *config = [WKWebViewConfiguration new];
	  WKUserContentController *ucc = [WKUserContentController new];
	  PMMsgProxy *proxy = [PMMsgProxy new];
	  proxy.windowID = wid;
	  [ucc addScriptMessageHandler:proxy name:@"pm"];
	  NSString *bootJS =
	      @"(function(){window.__pmCall=function(name,args){var "
	      @"list=[];if(args){for(var i=0;i<args.length;i++){var "
	      @"v=args[i];list.push(v===undefined||v===null?'':String(v));}}try{window.webkit."
	      @"messageHandlers.pm.postMessage({name:String(name),args:list});}catch(e){}};})();";
	  WKUserScript *boot = [[WKUserScript alloc] initWithSource:bootJS
						      injectionTime:WKUserScriptInjectionTimeAtDocumentStart
						       forMainFrameOnly:YES];
	  [ucc addUserScript:boot];
	  config.userContentController = ucc;

	  WKWebView *wk = [[WKWebView alloc] initWithFrame:window.contentView.bounds configuration:config];
	  wk.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
	  if (isFloat) {
		  wk.wantsLayer = YES;
		  wk.layer.cornerRadius = 16.0;
		  wk.layer.masksToBounds = YES;
		  [wk setValue:@NO forKey:@"drawsBackground"];
		  if (@available(macOS 10.14, *)) {
			  wk.underPageBackgroundColor = [NSColor clearColor];
		  }
	  }
	  if (flags & PM_WINDOW_DEBUG) {
		  if (@available(macOS 13.3, *)) {
			  [wk setValue:@YES forKey:@"inspectable"];
		  }
	  }
	  window.contentView = wk;

	  PMWindowCtl *ctl = [PMWindowCtl new];
	  ctl.windowID = wid;
	  ctl.flags = flags;
	  ctl.window = window;
	  ctl.webView = wk;
	  ctl.proxy = proxy;
	  window.delegate = ctl;
	  wk.navigationDelegate = ctl;
	  gWindows[@(wid)] = ctl;
	  outID = wid;
	});
	return outID;
}

void PMWindowNavigate(int id, const char *url) {
	if (!url) {
		return;
	}
	NSString *nsURL = [NSString stringWithUTF8String:url];
	PMOnMain(^{
	  PMWindowCtl *ctl = PMGet(id);
	  if (!ctl) {
		  return;
	  }
	  NSURL *u = [NSURL URLWithString:nsURL];
	  if (!u) {
		  return;
	  }
	  [ctl.webView loadRequest:[NSURLRequest requestWithURL:u]];
	});
}

void PMWindowEval(int id, const char *js) {
	if (!js) {
		return;
	}
	NSString *nsJS = [NSString stringWithUTF8String:js];
	PMOnMain(^{
	  PMWindowCtl *ctl = PMGet(id);
	  if (!ctl) {
		  return;
	  }
	  [ctl.webView evaluateJavaScript:nsJS completionHandler:nil];
	});
}

void PMWindowSetBindScript(int id, const char *js) {
	NSString *nsJS = js ? [NSString stringWithUTF8String:js] : @"";
	PMOnMain(^{
	  PMWindowCtl *ctl = PMGet(id);
	  if (!ctl) {
		  return;
	  }
	  ctl.bindJS = nsJS;
	  [ctl applyBindScript];
	});
}

void PMWindowShow(int id) {
	PMOnMain(^{
	  PMWindowCtl *ctl = PMGet(id);
	  if (!ctl) {
		  return;
	  }
	  [NSApp activateIgnoringOtherApps:YES];
	  [ctl.window makeKeyAndOrderFront:nil];
	});
}

void PMWindowHide(int id) {
	PMOnMain(^{
	  PMWindowCtl *ctl = PMGet(id);
	  if (!ctl) {
		  return;
	  }
	  [ctl.window orderOut:nil];
	});
}

bool PMWindowIsVisible(int id) {
	__block bool vis = false;
	PMOnMain(^{
	  PMWindowCtl *ctl = PMGet(id);
	  vis = ctl && ctl.window.isVisible;
	});
	return vis;
}

void PMWindowPlaceBottomRight(int id, int width, int height) {
	PMOnMain(^{
	  PMWindowCtl *ctl = PMGet(id);
	  if (!ctl) {
		  return;
	  }
	  NSScreen *screen = ctl.window.screen ?: [NSScreen mainScreen];
	  NSRect vf = screen.visibleFrame;
	  CGFloat w = width > 0 ? width : NSWidth(ctl.window.frame);
	  CGFloat h = height > 0 ? height : NSHeight(ctl.window.frame);
	  CGFloat x = NSMaxX(vf) - w - 16;
	  CGFloat y = NSMinY(vf) + 16;
	  [ctl.window setFrame:NSMakeRect(x, y, w, h) display:YES];
	  ctl.window.level = NSFloatingWindowLevel;
	});
}

void PMWindowEnsureTopmost(int id) {
	PMOnMain(^{
	  PMWindowCtl *ctl = PMGet(id);
	  if (!ctl) {
		  return;
	  }
	  ctl.window.level = NSFloatingWindowLevel;
	  [ctl.window orderFrontRegardless];
	});
}

void PMWindowDestroy(int id) {
	PMOnMain(^{
	  PMWindowCtl *ctl = PMGet(id);
	  if (!ctl) {
		  return;
	  }
	  ctl.flags &= ~PM_WINDOW_HIDE_ON_CLOSE;
	  [ctl.webView.configuration.userContentController removeScriptMessageHandlerForName:@"pm"];
	  ctl.window.delegate = nil;
	  ctl.webView.navigationDelegate = nil;
	  [ctl.window close];
	  [gWindows removeObjectForKey:@(id)];
	});
}

void PMTrayStart(const uint8_t *rgba, int width, int height) {
	NSImage *img = PMImageFromRGBA(rgba, width, height);
	if (img) {
		img.size = NSMakeSize(18, 18);
		img.template = NO;
	}
	PMOnMain(^{
	  if (gStatusItem) {
		  return;
	  }
	  gTrayTarget = [PMTrayTarget new];
	  gStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
	  gStatusItem.button.image = img;
	  gStatusItem.button.imageScaling = NSImageScaleProportionallyDown;
	  gStatusItem.button.toolTip = @"PhraseMate · 后台运行中";

	  NSMenu *menu = [NSMenu new];
	  NSMenuItem *openItem = [[NSMenuItem alloc] initWithTitle:@"打开生词本"
							    action:@selector(onMenu:)
						     keyEquivalent:@""];
	  openItem.target = gTrayTarget;
	  openItem.tag = PM_TRAY_OPEN;
	  [menu addItem:openItem];

	  NSMenuItem *floatItem = [[NSMenuItem alloc] initWithTitle:@"显示速记窗"
							     action:@selector(onMenu:)
						      keyEquivalent:@""];
	  floatItem.target = gTrayTarget;
	  floatItem.tag = PM_TRAY_FLOAT;
	  [menu addItem:floatItem];

	  NSMenuItem *scItem = [[NSMenuItem alloc] initWithTitle:@"创建桌面快捷方式"
							  action:@selector(onMenu:)
						   keyEquivalent:@""];
	  scItem.target = gTrayTarget;
	  scItem.tag = PM_TRAY_SHORTCUT;
	  [menu addItem:scItem];

	  [menu addItem:[NSMenuItem separatorItem]];

	  NSMenuItem *exitItem = [[NSMenuItem alloc] initWithTitle:@"退出 PhraseMate"
							    action:@selector(onMenu:)
						     keyEquivalent:@"q"];
	  exitItem.target = gTrayTarget;
	  exitItem.tag = PM_TRAY_EXIT;
	  [menu addItem:exitItem];

	  gStatusItem.menu = menu;
	});
}

void PMTrayStop(void) {
	PMOnMain(^{
	  if (!gStatusItem) {
		  return;
	  }
	  [[NSStatusBar systemStatusBar] removeStatusItem:gStatusItem];
	  gStatusItem = nil;
	  gTrayTarget = nil;
	});
}
