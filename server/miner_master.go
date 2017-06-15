package main

type Master struct {
}

func NewMaster() (m *Master) {
    return &Master{}
}

func (m *Master) Mainloop() {
}
