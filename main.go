package main

import (
    "fmt"
    "flag"
    "os"
    ol "github.com/ossrs/go-oryx-lib/logger"
)

const (
    version = "0.0.1"

    SRS_MP4_EOF_SIZE = 0
    SRS_MP4_USE_LARGE_SIZE = 1

    SRS_MP4_BUF_SIZE = 4096
)

func main()  {
    fmt.Println(fmt.Sprintf("mp4 parser:%v, by panda of bravovcloud.com", version))

    var mp4Url string
    flag.StringVar(&mp4Url, "url", "./test.mp4", "mp4 file to be parsed")

    ol.T(nil, "the input mp4 url is:", mp4Url)

    var f * os.File
    var err error
    if f, err = os.Open(mp4Url); err != nil {
        ol.T(nil, fmt.Sprint("open file:%v failed, err is %v", mp4Url, err))
        return
    }

    for {
        mb := NewMp4Box()
        var box Box
        if box, err = mb.discovery(f); err != nil {
            ol.E(nil, fmt.Sprintf("discovery box failed, err is %v", err))
            return
        }
        ol.T(nil, fmt.Sprintf("main discovery box:%+v", box))

        if err = box.DecodeHeader(f); err != nil {
            ol.E(nil, fmt.Sprintf("mp4 decode contained box header failed, err is %v", err))
            return
        }
        ol.T(nil, fmt.Sprintf("main after decode header, box:%+v", box))

        if err = box.Basic().DecodeBoxes(f); err != nil {
            ol.E(nil, fmt.Sprintf("mp4 decode contained box boxes failed, err is %v", err))
            return
        }
    }
}