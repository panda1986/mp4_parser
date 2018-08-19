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
--- free
--- free    
--- mdat

```    
         
#### ftyp

* 一般放置在文件的开始位置，
```
    majorBrand uint32
    minorVersion uint32
    compatibleBrands []uint32
```                              

#### moov

* 该box包含了文件媒体的metadata信息，该box有且只有1个，只被包含在文件层
* 子boxes

    * mvhd
        ```
        create_time
        time_scale(时间单位)
        duration
        volume
        rate
        ```
        
    * trak （有1个或多个,视频或者音频）
        * tkhd
        ```
        track_ID: 不会重复，而且不能是0
        duration: track的时长
        volume: track的音频volume
        width， height: track的分辨率
        ```
        
        * mdia
            * mdhd
            * hdlr
            
            ```
            handler_type: 指示trak的类型，vide, soun, hint 

            ```
            * minf
                * stbl sample table box(包含track的所有samples的time和data索引信息)
                    * stsd sample description, 音视频编码信息
                    * stts 说明该track总的有多少帧，总的时长
                    * stss 该box确定media中的关键帧帧号，主要用于视频拖动
                    * stsc 存储了chunk与sample的映射关系，一个chunk含有多少个sample
                    * stsz 该box确定了每一帧的大小
                    * stco 定义了每个chunk在媒体文件中的偏移位置
* 为什么把moov放在文件头部可以有fast start的效果？(stss)

```
You can see the browser makes 3 requests before it can start playing the video. 
In the first request, the browser downloads the first 552 KB of the video using an [HTTP range request](https://en.wikipedia.org/wiki/Byte_serving). 
We can tell this by the 206 Partial Content HTTP response code, and by digging in and looking at the request headers. However the moov
 atom is not there so the browser cannot start to play the video. 
 Next, the browser requests the final 21 KB of the video file using another range request. This does contain the moov
 atom, telling the browser where the video and audio streams start. 
 Finally, the browser makes a third and final request to get the audio/video data and can start to play the video. 
 This has wasted over half a megabyte of bandwidth and delayed the start of the video by 210 ms! Simply because the browser couldn’t find the moov
 atom.
```
* stco中第一个chunck的offset，就是mdat data的起始位置

#### mdat

* 该box包含媒体数据(可能有多个)

### 如何解析一个box

* 4字节的size 
    * 如果size == 1, 读取8字节的large size
    * 如果size == 0, 表明是文件的最后一个box,通常用于mdat box
* 4字节的type 
    * //根据type判断box类型
    * 如果类型不明可以跳过此box
