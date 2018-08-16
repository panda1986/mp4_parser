# mp4_parser
parse mp4 file in json format, written by golang
input: can be local mp4 file or http url, such as "http://xxxx.mp4"
output: json format to stdout and web api


## build

1. cd mp4_parser
1. go build .
1. ./mp4_parser

> 代码写完之后丢一边了，自己感觉都没有什么价值，还是应该写一下深刻的理解与说明，不枉费自己花费这么些时间与精力来解析这个复杂的box套box结构
    
## 概述：

* MP4文件的所有数据都装在box中，也就是说MP4文件由若干个box组成，。
* box中还可以嵌套子box, 这种box称为container box.

### 术语

* Box
    * box有类型和长度，不管在代码中还是在象形的理解中，可以将box看做一个class，一个抽象的对象
    * 其他规范可能有叫atom的

* Chunk
    * A continouts set of samples of one track,一个track的连续samples组成集合
    
* Container Box
    * box的一种，是单纯为了包含一系列相关的boxes而存在
    
* Hint Track
    * 一种特殊的track, 不包含media data. 相反它包含将一条或多条tracks打包成一条流的指令.

* Hinter:
    * 工具

* Movie Box:
    * moov, 是一种container box, 它的子boxes是关于metadata相关的
    
* Media Box Data:
    * mdat, 是一种container box, 它存储的是实际的media data
    
* Sample:
    * 在non-hint track, a video sample就是一个单独的视频帧数据,或一组连续的视频帧，audio sample为一段连续的音频压缩数据。
    
* Sample Table:
    * 指明sample 时序和物理布局的表
    

### 结构树

```
--- ftyp
--- moov
    --- mvhd
    --- trak
    --- trak
    --- trak
    --- trak
--- free
--- free    
--- mdat

```    
                           

### 如何解析一个box