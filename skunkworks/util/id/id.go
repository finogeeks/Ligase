package id

import (
	"github.com/bwmarrin/snowflake"
	"net"

	"errors"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

var (
	node   = new()
	nodeId int64
)

func Next() int64 {
	return node.Generate().Int64()
}

func NextSeq() string {
	return node.Generate().String()
}

func GetNodeId() int64 {
	return nodeId
}

func new() *snowflake.Node {
	nodeId = 100
	rand.Seed(time.Now().UnixNano())

	//get node id from ip address
	ip, err := hostIP()
	if err == nil && ip != "" {
		nodeId = ip2num(ip) % 1023
	} else {
		//use random id if no ip
		nodeId = rand.Int63() % 1023
	}

	nd, err := snowflake.NewNode(nodeId)
	if err != nil {
		nd, _ = snowflake.NewNode(100)
	}

	return nd
}

func hostIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}

func ip2num(ip string) int64 {
	canSplit := func(c rune) bool { return c == '.' }
	lisit := strings.FieldsFunc(ip, canSplit) //[58 215 20 30]
	ip1_str_int, _ := strconv.Atoi(lisit[0])
	ip2_str_int, _ := strconv.Atoi(lisit[1])
	ip3_str_int, _ := strconv.Atoi(lisit[2])
	ip4_str_int, _ := strconv.Atoi(lisit[3])
	num := ip1_str_int<<24 | ip2_str_int<<16 | ip3_str_int<<8 | ip4_str_int
	return int64(num)
}
