# PhraseMate
感觉市面上当前已经有的单词本很难用，很多短语都无法记录，于是用cursor生成一个简单的单词本小程序，可以接入api，在边看B站网课的时候边听边记录

技术栈：**Go** + **原生窗口**（Windows: WebView2 / macOS: WKWebView）+ **SQLite** + **OpenAI 兼容** API。

## 功能

- **桌面窗口启动**：双击 / 命令行启动即打开应用窗口（不再依赖浏览器）
- **桌面快捷方式**：打包后首次启动自动创建，也可从托盘菜单一键生成
- **系统置顶速记窗**：独立于主界面，可叠在 B 站等全屏画面角落录入
- **界面填写 API Key**：设置里保存即可，无需手改 `.env`
- **AI 双语释义**：英文释义、中文释义、音标、词性、例句
- **词典优先**：常见单词先查免费词典，未命中再调用 AI（省额度、更快）
- **生词本**：自动去重更新，支持筛选与删除
- **自测**：根据生词本生成四选一选择题并即时判分

## 普通用户（推荐）

### Windows

1. 打开 [Releases](https://github.com/Lucas2029460802/PhraseMate/releases) ，下载最新的 **PhraseMate.exe**
2. 双击运行（首次会尝试创建桌面快捷方式）
3. 打开右上角 **设置**，填写 API Key（可选 Base URL / 模型）
4. 保存后即可用速记窗收录单词

> 不需要安装 Go，也不需要创建或编辑 `.env`。配置保存在本地 `data/phrasemate.db`。

Windows 10/11 一般已自带 [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)。若窗口创建失败，请安装该运行时。

### macOS

目前请从源码构建（见下方「开发者」）。首次运行会：

- 使用系统自带的 **WKWebView** 打开窗口（无需 WebView2）
- 在菜单栏显示 PhraseMate 图标（关闭主窗口即隐藏到菜单栏）
- 尝试把应用安装到 `~/Applications/PhraseMate.app`，并在桌面创建别名
- 把数据库写到 `~/Library/Application Support/PhraseMate/phrasemate.db`

需要已安装 [Xcode Command Line Tools](https://developer.apple.com/xcode/)（`xcode-select --install`）。

### 设置示例

| 服务 | Base URL | 模型示例 |
|------|----------|----------|
| OpenAI | `https://api.openai.com/v1` | `gpt-4o-mini` |
| DeepSeek | `https://api.deepseek.com/v1` | `deepseek-chat` |

设置面板里也有「OpenAI / DeepSeek」快捷填充。

## 开发者：从源码运行

需要本机已安装 [Go](https://go.dev/dl/)（仅开发/打包时需要）。

### Windows

```powershell
go mod tidy
go run .
```

会弹出 **PhraseMate** 应用窗口。关闭窗口即隐藏到托盘；退出请用托盘菜单。

### macOS

```bash
xcode-select --install   # 若尚未安装
go mod tidy
go run .
```

会弹出 **PhraseMate** 应用窗口。关闭窗口即隐藏到菜单栏；退出请用菜单栏图标。

> `go run` 是临时二进制，不会创建桌面快捷方式。需要快捷方式请先 `go build`。
>
> 可选：仍可用 `.env` 预填 Key（适合开发）。界面里保存的配置会写入数据库，并优先于 `.env`。

### 可选：浏览器模式

调试页面时：

Windows:

```powershell
$env:PHRASEMATE_WEB="1"; go run .
```

macOS / Linux:

```bash
PHRASEMATE_WEB=1 go run .
```

然后手动打开终端里打印的本地地址。

## 打包成软件

### Windows

无黑框控制台的桌面程序：

```powershell
go build -ldflags="-H windowsgui -s -w" -o PhraseMate.exe .
```

把 `PhraseMate.exe` 发给用户即可；用户在界面填写 API Key，无需附带 `.env`。

### macOS

必须在 Mac 本机构建（依赖 Cocoa / WebKit，不能从 Windows 交叉编译）：

```bash
CGO_ENABLED=1 go build -ldflags="-s -w" -o PhraseMate .
./PhraseMate --install-shortcut
```

会生成 `~/Applications/PhraseMate.app`，并在桌面创建别名。把 `.app` 发给其他 Mac 用户即可。

> 从 Windows 交叉编译 `GOOS=darwin` **不能** 得到可用的桌面程序（CGO + 系统 WebKit）。

### 桌面快捷方式

打包后任选其一：

1. **运行一次**：若桌面还没有快捷方式，会自动创建（Windows: `PhraseMate.lnk`；macOS: 桌面别名）
2. **托盘 / 菜单栏右键** →「创建桌面快捷方式」（可随时重建）
3. **命令行**：

Windows:

```powershell
.\PhraseMate.exe --install-shortcut
```

macOS:

```bash
./PhraseMate --install-shortcut
```

> 用 `go run .` 调试时不会创建快捷方式（临时路径无效）；请先 `go build`。

### 可选：环境变量（高级）

复制 `.env.example` 为 `.env` 可覆盖默认项；日常使用更推荐界面设置。

## 使用提示

1. 用右下角「速记」悬浮窗收录单词 / 短语
2. 同一词再次查询会更新释义
3. 生词本至少 2 条后可生成自测题
4. 未配置 Key 时仍可收录；常见单词走免费词典，其余为基础占位释义
