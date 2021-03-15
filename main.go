package main

import (
    "flag"
    "fmt"
    log "github.com/sirupsen/logrus"
    "io"
    "os"
    "panda.com/mp4parser/core"
    "path"
    "runtime"
    "time"
)

func Hello()  {
    
}

func main()  {
    log.SetLevel(log.DebugLevel)
    log.SetReportCaller(true)
    log.SetFormatter(&log.TextFormatter{
        FullTimestamp: true,
        CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
            fileName := path.Base(frame.File)
            return fmt.Sprintf("%s()", frame.Function), fmt.Sprintf("%s:%v", fileName, frame.Line)
        },
    })

    log.Infof("hi, good girl, welcome to this program, now is:%v", time.Now().Format("2006-01-02 15:04:05.999"))

    var inputUrl string
    flag.StringVar(&inputUrl, "i", "", "use -i to specify input url")
    flag.Parse()
    if len(inputUrl) == 0 {
        flag.PrintDefaults()
        log.Errorf("input url is empty")
        return
    }

    f, err := os.Open(inputUrl)
    if err != nil {
        log.Errorf("open input url:%v failed, err is %v", inputUrl, err)
        return
    }
    defer f.Close()



    for {
        mb := core.NewMp4Box()
        var box core.Box
        if box, err = mb.Discovery(f); err != nil {
            log.Errorf("discovery box failed, err is %v", err)
            break
        }
        log.Tracef("main discovery box:%+v", box)

        if err = box.DecodeHeader(f); err != nil {
            log.Errorf("mp4 decode contained box header failed, err is %v", err)
            break
        }
        log.Tracef("main after decode header, box:%+v", box)

        if err = box.Basic().DecodeBoxes(f); err != nil {
            log.Errorf("mp4 decode contained box boxes failed, err is %v", err)
            break
        }
    }

    if err == io.EOF {
        log.Tracef("decode mp4 file:%v success", inputUrl)
    }
}