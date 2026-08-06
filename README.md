# PhraseMate
感觉市面上当前已经有的单词本很难用，很多短语都无法记录，于是用cursor生成一个简单的单词本小程序，可以接入api，在边看B站网课的时候边听边记录

技术栈：**Go** + **WebView2** 原生窗口 + **SQLite** + **OpenAI 兼容** API。

## 功能

- **桌面窗口启动**：双击 / 命令行启动即打开应用窗口（不再依赖浏览器）
- **系统置顶速记窗**：独立于主界面，可叠在 B 站等全屏画面角落录入
- **AI 双语释义**：英文释义、中文释义、音标、词性、例句
- **词典优先**：常见单词先查免费词典，未命中再调用 AI（省额度、更快）
- **生词本**：自动去重更新，支持筛选与删除
- **自测**：根据生词本生成四选一选择题并即时判分

## 快速开始

### 1. 配置 API Key

```powershell
copy .env.example .env
```

编辑 `.env`：

```env
PHRASEMATE_API_KEY=sk-xxx
PHRASEMATE_BASE_URL=https://api.openai.com/v1
PHRASEMATE_MODEL=gpt-4o-mini
```

DeepSeek 示例：

```env
PHRASEMATE_API_KEY=sk-xxx
PHRASEMATE_BASE_URL=https://api.deepseek.com
PHRASEMATE_MODEL=deepseek-chat
```

### 2. 启动（桌面窗口）

```powershell
go mod tidy
go run .
```

会弹出 **PhraseMate** 应用窗口。关闭窗口即退出。

> Windows 10/11 一般已自带 [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)。若窗口创建失败，请安装该运行时。

### 3. 打包成软件

无黑框控制台的桌面程序：

```powershell
go build -ldflags="-H windowsgui -s -w" -o PhraseMate.exe .
```

把 `PhraseMate.exe` 与 `.env` 放在同一目录即可分发使用。

### 可选：浏览器模式

调试页面时：

```powershell
$env:PHRASEMATE_WEB="1"; go run .
```

然后手动打开终端里打印的本地地址。

## 使用提示

1. 用右下角「速记」悬浮窗收录单词 / 短语
2. 同一词再次查询会更新释义
3. 生词本至少 2 条后可生成自测题


```
