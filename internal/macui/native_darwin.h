#pragma once

#include <stdbool.h>
#include <stdint.h>

enum {
	PM_WINDOW_FLOAT = 1 << 0,
	PM_WINDOW_HIDE_ON_CLOSE = 1 << 1,
	PM_WINDOW_CENTER = 1 << 2,
	PM_WINDOW_DEBUG = 1 << 3
};

void PMInit(void);
void PMRun(void);
void PMQuit(void);
void PMSetAppIconRGBA(const uint8_t *rgba, int width, int height);

int PMWindowCreate(const char *title, int width, int height, int flags);
void PMWindowNavigate(int id, const char *url);
void PMWindowEval(int id, const char *js);
void PMWindowSetBindScript(int id, const char *js);
void PMWindowShow(int id);
void PMWindowHide(int id);
bool PMWindowIsVisible(int id);
void PMWindowPlaceBottomRight(int id, int width, int height);
void PMWindowEnsureTopmost(int id);
void PMWindowDestroy(int id);

void PMTrayStart(const uint8_t *rgba, int width, int height);
void PMTrayStop(void);
