package main

import (
    "io"
    "io/ioutil"
    "encoding/binary"
    "hash/crc32"
    "errors"
)

func CRCSave(filename string, msg string) error {
    byteMsg := []byte(msg)
    err := ioutil.WriteFile(filename, byteMsg, 0644)
    if err != nil {
        return err
    }
    bs := make([]byte, 4)
    binary.LittleEndian.PutUint32(bs, crc32.ChecksumIEEE(byteMsg))
    err = ioutil.WriteFile(filename + ".crc", bs, 0644)
    if err != nil {
        return err
    }
    return nil
}

func CRCSaveStream(w io.Writer, msg []byte) error {
    bs := make([]byte, 4)
    binary.LittleEndian.PutUint32(bs, uint32(len(msg)))
    _, err := w.Write(bs)
    if err != nil{
        return err
    }
    _, err = w.Write(msg)
    if err != nil{
        return err
    }
    binary.LittleEndian.PutUint32(bs, crc32.ChecksumIEEE(msg))
    _, err = w.Write(bs)
    if err != nil{
        return err
    }
    return nil
}

func CRCLoad(filename string) (msg string, err error) {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        return
    }
    buf, err := ioutil.ReadFile(filename + ".crc")
    if err != nil {
        return
    }
    crc := binary.LittleEndian.Uint32(buf)
    if crc != crc32.ChecksumIEEE(data) {
        err = errors.New("Checksum fail.")
    }
    msg = string(data)
    return
}

func CRCLoadStream(r io.Reader) (msg []byte, haveEOF bool, err error) {
    haveEOF = false
    bs := make([]byte, 4)
    _, err = r.Read(bs)
    if err != nil {
        if err == io.EOF {
            haveEOF = true
        }
        return
    }
    l := binary.LittleEndian.Uint32(bs)

    msg = make([]byte, l)
    _, err = r.Read(msg)
    if err != nil {
        return
    }

    _, err = r.Read(bs)
    if err != nil {
        return
    }

    crc := binary.LittleEndian.Uint32(bs)
    err = nil
    if crc != crc32.ChecksumIEEE(msg) {
        err = errors.New("Checksum fail.")
    }
    return
}

