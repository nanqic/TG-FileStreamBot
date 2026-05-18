<h1 align="center">Telegram File Stream Bot</h1>
<p align="center">
  </a>
  <p align="center">
    <a herf="https://github.com/EverythingSuckz/TG-FileStreamBot">
        <img src="https://filestream.bot/logo.svg" height="100" width="100" alt="File Stream Bot Logo">
    </a>
</p>
  <p align="center">
    A Telegram bot to <b>generate direct link</b> for your Telegram files.
    <br />
    << <a href="https://filestream.bot/getting-started">Visit docs</a> |  <a href="https://filestream.bot/support">Support</a> >>
  </p>
</p>

## Credits

- [@celestix](https://github.com/celestix) for [gotgproto](https://github.com/celestix/gotgproto)
- [@divyam234](https://github.com/divyam234/teldrive) for his [Teldrive](https://github.com/divyam234/teldrive) Project
- [@krau](https://github.com/krau) for adding image support

## Copyright

Copyright (C) 2026 [EverythingSuckz](https://github.com/EverythingSuckz) under [GNU Affero General Public License](https://www.gnu.org/licenses/agpl-3.0.en.html).

TG-FileStreamBot is Free Software: You can use, study share and improve it at your
will. Specifically you can redistribute and/or modify it under the terms of the
[GNU Affero General Public License](https://www.gnu.org/licenses/agpl-3.0.en.html) as
published by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version. Also keep in mind that all the forks of this repository MUST BE OPEN-SOURCE and MUST BE UNDER THE SAME LICENSE.

## 交叉编译 Linux AMD64 版本

```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"; go build -trimpath -ldflags="-s -w -extldflags '-static'" -o fsb-linux-amd64 ./cmd/fsb/
```

**命令说明：**

- `$env:GOOS="linux"`：指定目标操作系统为 Linux。
- `$env:GOARCH="amd64"`：指定目标架构为 x86_64 (AMD64)。
- `$env:CGO_ENABLED="0"`：禁用 CGo 编译器，确保生成的二进制文件为静态链接，不依赖宿主机环境的 C 库。
- `-trimpath`：移除文件路径信息，减小二进制文件体积并保护源码结构。
- `-ldflags="-s -w -extldflags '-static'"`：
  - `-s`：移除符号表，减小文件体积。
  - `-w`：移除DWARF调试信息，减小文件体积。
  - `-extldflags '-static'`：确保链接器使用静态链接选项，生成的二进制文件包含所有依赖库，可直接在目标 Linux 环境运行。
- `-o tg-fsb-linux-amd64`：指定生成的二进制输出文件名。

编译完成后，若需在当前终端会话中恢复默认的 Windows 编译环境，请执行以下命令清理临时环境变量：

```powershell
Remove-Item Env:\GOOS, Env:\GOARCH -ErrorAction SilentlyContinue

```
