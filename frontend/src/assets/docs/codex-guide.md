# Codex 部署和使用教程（Windows 版）

> 版本：加入 aitokey API 中转站的预览版  
> 适用对象：第一次在 Windows 电脑上安装和使用 Codex 的用户  

## 1. 教程目标

本教程按原始文档的步骤展开，目标是帮助用户完成 Codex 使用前的环境准备、Codex 安装、CC Switch 配置、aitokey API 中转站接入和基础使用验证。

完整顺序建议为：安装 Git、安装 Node.js、可选安装 VS Code、安装 Codex、了解 aitokey API 中转站、安装 CC Switch、在 CC Switch 中配置中转站密钥、重启 Codex 验证模型更新。

## 2. 准备材料

| 文件/网址 | 用途 | 建议 |
|---|---|---|
| Git-2.54.0-64-bit (1).exe | 安装 Git，提供代码仓库和命令行能力 | 建议默认路径安装 |
| node-v24.16.0-x64.msi | 安装 Node.js，提供 npm 和前端/CLI 运行环境 | 安装后用 `node -v` 验证 |
| CC-Switch-v3.15.0-Windows.msi | 安装 CC Switch，用于更新模型密钥或模型配置 | 安装路径可自定义 |
| https://aitokey.top | aitokey API 中转站入口 | 进入站内文档查看接入参数 |
| VS Code | 代码编辑器 | 编程场景不多可以暂不安装 |

## 3. 安装 Git

1. 打开 `安装包/Git-2.54.0-64-bit (1).exe`。
2. 按安装向导默认选项安装即可。
3. 不建议修改 Git 的安装路径。原始文档特别提醒，如果修改路径，后续可能出现工具扫描不到 Git 的情况。
4. 安装完成后，打开命令提示符验证：

```bat
git --version
```

如果能看到类似 `git version 2.54.0.windows.1` 的版本号，说明 Git 安装成功。

## 4. 安装 Node.js

1. 打开 `安装包/node-v24.16.0-x64.msi`。
2. 按默认向导安装即可。
3. 安装完成后按 `Win + R`，输入 `cmd`，回车。
4. 在命令提示符中输入：

```bat
node -v
```

如果返回版本号，例如 `v24.16.0`，说明 Node.js 已安装成功。

建议再验证 npm：

```bat
npm -v
```

如果 npm 也返回版本号，说明后续安装命令行工具时环境基本可用。

## 5. VS Code 可选安装

VS Code 是代码编辑器，不是 Codex 运行的唯一前置条件。原始文档建议：如果编程场景不多，可以先不安装；需要时再从官网下载安装。

如果需要安装，建议使用默认安装路径，并勾选“添加到 PATH”或“通过 Code 打开”一类选项，便于后续从项目文件夹启动编辑器。

## 6. 安装 Codex App

原始文档要求：去微软应用商城搜索 Codex 并下载。

建议步骤如下：

1. 打开 Windows 的 Microsoft Store。
2. 在搜索框输入 `Codex`。
3. 找到 OpenAI Codex 相关应用后点击安装。
4. 安装完成后从开始菜单启动 Codex。
5. 按提示登录 OpenAI/ChatGPT 账号。

如果 Microsoft Store 搜索不到，可以先打开 OpenAI 官方 Codex 入门页确认当前可用入口，再回到商店安装。

官方入口参考：<https://openai.com/codex/get-started/>

## 7. 接入 aitokey API 中转站

aitokey API 网址：https://aitokey.top

aitokey API 是一个稳定、统一的 AI API 接入平台。根据首页介绍，它提供模型调用、密钥管理、用量统计与迁移支持，适合把不同工具统一接入到一个兼容入口中，方便后续监控用量、切换模型和管理密钥。

首页展示的接入思路是 3 步：

1. 登录账号：进入控制台查看可用额度、服务状态和调用记录。
2. 创建 API Key：按项目生成调用密钥，便于权限隔离、用量追踪和风险控制。
3. 配置调用参数：按站内文档配置 Base URL、模型名称和请求参数，即可发起首次调用。

具体接入步骤以站内文档为准。当前文档入口需要登录后访问，建议登录 aitokey API 后点击顶部“文档”，复制站内给出的 Base URL、模型名称和 API Key，再填入 CC Switch 或对应客户端。

注意：API Key 属于敏感信息，不建议截图、转发或写入教程正文。给他人演示时可以只展示 Key 的前后几位，或使用临时测试密钥。

## 8. 安装 CC Switch

1. 打开 `安装包/CC-Switch-v3.15.0-Windows.msi`。
2. 按向导安装。
3. 这一项原始文档说明“路径可以改到自己想改的目录里”，因此可以放在常用软件目录，例如 `D:/Tools/CC-Switch`。
4. 安装完成后启动 CC Switch。

## 9. 在 CC Switch 中配置 aitokey API 并重启 Codex

原始文档特别提醒：在 CC Switch 更新模型密钥之后，需要关闭 Codex 并重新打开，模型才会自动更新，而且一定要关闭进程。

建议操作：

1. 打开 aitokey API：https://aitokey.top
2. 登录账号后进入控制台，创建或复制 API Key。
3. 查看接入所需的 Base URL、模型名称和请求参数。
4. 打开 CC Switch，把 aitokey API 的 Base URL、API Key 和模型信息填入对应配置位置。
5. 保存配置或执行模型密钥更新。
6. 关闭 Codex 窗口。
7. 打开任务管理器，确认 Codex 相关进程已经结束。
8. 重新打开 Codex。
9. 新建一个对话或任务，检查模型是否已经更新。

## 10. Codex 基础使用流程

### 10.1 打开项目文件夹

Codex 面向真实项目工作。建议先准备一个项目文件夹，例如：

```text
D:/Projects/my-demo-project
```

在 Codex 中选择该文件夹作为工作区。第一次进入项目时，可以先让 Codex 阅读目录结构，再提出具体任务。

### 10.2 推荐的提问方式

好的任务描述通常包含目标、范围、限制和验收方式。例如：

```text
请帮我检查这个项目为什么启动失败。先不要改代码，先读 package.json 和报错日志，然后告诉我原因。
```

或者：

```text
请在不改变现有页面风格的前提下，给登录页增加“记住我”复选框，并运行现有测试。
```

### 10.3 常用使用场景

| 场景 | 可以这样问 |
|---|---|
| 读代码 | “请解释这个项目的启动流程和主要模块。” |
| 修 bug | “请根据这段报错定位原因并修复。” |
| 写功能 | “请按现有代码风格新增这个功能。” |
| 代码审查 | “请 review 最近改动，重点看潜在 bug 和缺少测试的地方。” |
| 写文档 | “请根据当前代码生成部署说明。” |

## 11. 常见问题

### 11.1 `node -v` 没有返回版本号

可能原因是 Node.js 没安装成功，或者 PATH 没刷新。可以先关闭命令提示符重新打开，再执行 `node -v`。如果仍失败，重新安装 Node.js，并尽量保留默认选项。

### 11.2 `git --version` 没有返回版本号

可能原因是 Git 未加入 PATH，或者安装路径被修改后没有被系统识别。建议重新运行 Git 安装包，优先保留默认安装路径。

### 11.3 CC Switch 更新后 Codex 没变化

重点检查 Codex 是否真的退出。只关闭窗口不一定代表进程完全结束，建议在任务管理器中结束 Codex 相关进程后再重新打开。

### 11.4 aitokey API 配好后仍无法调用

优先检查三项：Base URL 是否和站内文档一致，API Key 是否复制完整，模型名称是否和站内可用模型列表一致。如果仍失败，登录 aitokey API 控制台查看调用记录、余额和错误信息。

### 11.5 是否必须安装 VS Code

不是必须。VS Code 主要用于人工查看和编辑代码。只做少量 Codex 任务时可以不装；如果经常开发，建议安装。

## 12. 验收清单

- [ ] Git 安装完成，`git --version` 有版本号。
- [ ] Node.js 安装完成，`node -v` 有版本号。
- [ ] npm 可用，`npm -v` 有版本号。
- [ ] Codex App 可以正常打开并登录。
- [ ] aitokey API 可以正常登录并查看站内文档。
- [ ] 已在 aitokey API 创建或复制 API Key。
- [ ] CC Switch 已安装并完成 aitokey API 参数配置。
- [ ] Codex 已完全关闭并重新打开。
- [ ] 新建任务后模型配置生效。

## 13. 参考来源

- 原始步骤文档：`D:/zhumian/新建文件夹/新建 DOCX 文档.docx`
- OpenAI Codex 官方入门页：<https://openai.com/codex/get-started/>
- OpenAI Codex Windows 文档：<https://developers.openai.com/codex/windows>
- OpenAI Codex App 文档：<https://developers.openai.com/codex/app>
