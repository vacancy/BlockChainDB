package main

import "github.com/golang/protobuf/proto"

// Define the P2P response, which is a iterator.

type P2PResponse struct {
    channel chan proto.Message
    acquiredClose bool
}

func NewP2PResponse(bufferSize int) (r *P2PResponse) {
    return &P2PResponse{
        channel: make(chan proto.Message, bufferSize),
        acquiredClose: false,
    }
}

// Functions called by sender

func (r *P2PResponse) Push(msg proto.Message) {
    r.channel <- msg
}

func (r *P2PResponse) AcquiredClose() bool {
    return r.acquiredClose
}

func (r *P2PResponse) Close() {
    r.channel <- nil
}

// Functions called by receiver

func (r *P2PResponse) AcquireClose() {
    r.acquiredClose = true
}

func (r *P2PResponse) IgnoreLater() {
    go func() {
        for {
            msg := <-r.channel
            if msg == nil {
                break
            }
        }
    }()
}

func (r *P2PResponse) Get() proto.Message {
    if r.acquiredClose {
        return nil
    }

    msg := <-r.channel
    if msg == nil {
        r.acquiredClose = true
    }
    return msg
}
