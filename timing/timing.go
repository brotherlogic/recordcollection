package main

import (
	"fmt"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"github.com/brotherlogic/keystore/client"

	pbrc "github.com/brotherlogic/recordcollection/proto"
)

func getIP(server string) (string, int) {
	t := time.Now()
	h, p, _ := utils.Resolve(server)
	fmt.Printf("GOT %v\n", time.Now().Sub(t))
	return h, int(p)
}

func main() {
	t := time.Now()
	client := *keystoreclient.GetClient(getIP)
	rc := &pbrc.RecordCollection{}
	_, b, c := client.Read("/github.com/brotherlogic/recordcollection/collection", rc)
	fmt.Printf("TOOK %v -> %v,%v\n", time.Now().Sub(t), b.GetReadTime(), c)
}
