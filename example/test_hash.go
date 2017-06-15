package main

import (
    "fmt"
    "../hash"
)

func main() {
    for i:=0;i<999999;i++{
        nonce:=fmt.Sprintf("%08d",i)
        jsonEncoded:=`{
        "BlockID":1,
        "Nonce":"`+nonce+`"
        }`
        h:=hash.GetHashString(jsonEncoded)
        if hash.CheckHash(h){
            fmt.Println("Nonce:",nonce," Hash:",h)
        }
    }
}
