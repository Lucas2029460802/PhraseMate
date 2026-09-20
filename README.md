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

1. 打开 [Releases](https://github.com/Lucas2029460802/PhraseMate/releases) ，下载最新的 **PhraseMate.exe**（或 `PhraseMate-windows-*.exe`）
2. 双击运行（首次会尝试创建桌面快捷方式）
3. 打开右上角 **设置**，填写 API Key（可选 Base URL / 模型）
4. 保存后即可用速记窗收录单词

> 不需要安装 Go，也不需要创建或编辑 `.env`。配置保存在本地 `data/phrasemate.db`。
>
> Windows / macOS 安装包都由 GitHub Actions 自动构建。推送 `v*` 标签会同时发布两个平台的产物。

Windows 10/11 一般已自带 [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)。若窗口创建失败，请安装该运行时。

### macOS

1. 打开 [Releases](https://github.com/Lucas2029460802/PhraseMate/releases) ，下载最新的 **PhraseMate-macos-*.zip**
2. 解压得到 `PhraseMate.app`，拖到「应用程序」
3. 首次打开：在 Finder 里 **右键 → 打开**（未公证签名时系统会拦截，选一次即可）
4. 打开右上角 **设置**，填写 API Key（可选 Base URL / 模型）

> 配置和生词本保存在 `~/Library/Application Support/PhraseMate/phrasemate.db`。
>
> macOS 安装包是 Apple Silicon + Intel 通用二进制。

## Docker（Windows 网页实例）

原生桌面窗口、托盘和置顶速记窗无法在容器里运行。Docker 镜像走 **浏览器模式**：容器只提供网页服务，在 Windows 浏览器里使用。

### 前置条件

1. 安装 [Docker Desktop for Windows](https://www.docker.com/products/docker-desktop/)
2. 保持默认的 **Linux containers** 模式（托盘图标 → Switch to Linux containers）

### 构建并启动

在项目根目录 PowerShell：

```powershell
docker compose up -d --build
```

浏览器打开 [http://localhost:8080](http://localhost:8080) 。右上角 **设置** 填写 API Key 即可。

生词本默认保存在 Docker 数据卷 `phrasemate-data` 中，停止或重建容器不会丢数据。

### 常用命令

```powershell
docker compose logs -f          # 查看日志
docker compose stop             # 停止
docker compose start            # 再启动
docker compose down             # 删除容器（保留数据卷）
docker compose down -v          # 删除容器和生词本数据
```

### 只构建 / 运行镜像

```powershell
docker build -t phrasemate:latest .
docker run --name phrasemate -d -p 8080:8080 -v phrasemate-data:/data phrasemate:latest
```

若希望数据库文件出现在当前目录的 `data` 文件夹，把 `docker-compose.yml` 里的数据卷改成：

```yaml
volumes:
  - ./data:/data
```

容器内没有系统级置顶速记窗；可用主页面收录单词，或另开 `/float.html` 作为简易速记页。

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

GitHub Actions 的 Windows runner 会自动打出同样的无控制台 exe。

### macOS

必须在 Mac 本机构建（依赖 Cocoa / WebKit，不能从 Windows 交叉编译）：

```bash
CGO_ENABLED=1 go build -ldflags="-s -w" -o PhraseMate .
./PhraseMate --pack-app dist/PhraseMate.app
```

日常开发仍可用 `go run .`。发布请用 GitHub Actions：推送 `v1.0.0` 这类标签后，会同时构建：

- **Windows**：`PhraseMate-windows-*.exe`（amd64，无控制台）
- **macOS**：`PhraseMate-macos-*.zip`（arm64 + amd64 通用 `.app`）
- **Docker**：`phrasemate-docker.tar.gz`（Linux 网页实例镜像，可在 Windows Docker Desktop 导入）

也可在仓库的 **Actions → Build → Run workflow** 手动跑一次，产物在 Artifacts 里。

导入已构建的 Docker 镜像（需已安装 Docker Desktop）：

```powershell
docker load -i phrasemate-docker.tar.gz
docker run --name phrasemate -d -p 8080:8080 -v phrasemate-data:/data phrasemate:latest
```

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
