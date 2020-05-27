package id

import (
	"fmt"
	"testing"
)

func Test_Id(t *testing.T) {
	fmt.Println("get one id:", Next())
	fmt.Println("get node seq:", NextSeq())
}

func Test_Node(t *testing.T) {
	fmt.Println("get node id:", GetNodeId())
}

func Test_Hostip(t *testing.T) {
	ip, _ := hostIP()
	fmt.Println("get host ip:", ip)
	fmt.Println("get host ip num:", ip2num(ip))

}
