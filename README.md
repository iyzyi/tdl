<h2 align="center">iyzyi fork change log</h2>

PS: 该软件原本的功能就已十分强大，本fork是为了结合[iyzyi/tdl_wrap](https://github.com/iyzyi/tdl_wrap)更好地适应我自己的VPS挂机需求，不建议大家使用。

1. 禁用下载功能原本的断点恢复（通过bolt等文件保存）
2. 自行实现下载、转发功能的断点恢复（将在程序所在目录下创建`record/download|forward/{fromID}.txt`中存储`fromID`对应的聊天(频道、群组等)的已经下载过或转发过的`msgID`）
3. 下载时，如果所选消息中包含单个文件，则下载到`{dir}/{fromID}/original/{msgID}`；如果所选消息中包含合并文件（从`msgID1`到`msgID2`），则下载到`{dir}/{fromID}/original/{msgID1}-{msgID2}`。同时，如果带有文件的消息中含有文本信息，则保存到相应文件夹中的`info.txt`中。

<h1 align="center">tdl</h1>

<p align="center">
📥 Telegram Downloader, but more than a downloader
</p>

<p align="center">
English | <a href="README_zh.md">简体中文</a>
</p>

<p align="center">
<img src="https://img.shields.io/github/go-mod/go-version/iyear/tdl?style=flat-square" alt="">
<img src="https://img.shields.io/github/license/iyear/tdl?style=flat-square" alt="">
<img src="https://img.shields.io/github/actions/workflow/status/iyear/tdl/master.yml?branch=master&amp;style=flat-square" alt="">
<img src="https://img.shields.io/github/v/release/iyear/tdl?color=red&amp;style=flat-square" alt="">
<img src="https://img.shields.io/github/downloads/iyear/tdl/total?style=flat-square" alt="">
</p>

## Features

- Single file start-up
- Low resource usage
- Take up all your bandwidth
- Faster than official clients
- Download files from (protected) chats
- Forward messages with automatic fallback and message routing
- Upload files to Telegram
- Export messages/members/subscribers to JSON

## Preview

It reaches my proxy's speed limit, and the **speed depends on whether you are a premium**

![](img/preview.gif)

## Documentation

Please refer to the [documentation](https://docs.iyear.me/tdl/).

## Sponsors

![](https://raw.githubusercontent.com/iyear/sponsor/master/sponsors.svg)

## LICENSE

AGPL-3.0 License
