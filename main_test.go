package main

import (
    "encoding/binary"
    "fmt"
    "log"
    "testing"
)

func TestEndian(t *testing.T) {
    x := 256
    bigData := make([]byte, 4)
    littleData := make([]byte, 4)
    binary.BigEndian.PutUint32(bigData, uint32(x))
    binary.LittleEndian.PutUint32(littleData, uint32(x))
    log.Println(fmt.Sprintf("big endian data=%v, little endian data=%v", bigData, littleData))
}
