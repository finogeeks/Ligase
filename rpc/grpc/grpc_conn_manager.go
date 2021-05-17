// Copyright (C) 2020 Finogeeks Co., Ltd
//
// This program is free software: you can redistribute it and/or  modify
// it under the terms of the GNU Affero General Public License, version 3,
// as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package grpc

import (
	"errors"
	"sync"
	"sync/atomic"
	"unsafe"

	_ "github.com/mbobakov/grpc-consul-resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type GrpcConnectItem struct { //单个连接建立，例如：和账号系统多实例连接上，一个item可以理解为对应一个服务多实例的连接
	ClientConn unsafe.Pointer
	ServerName string
}

type GrpcConnectManager struct {
	CreateLock         sync.Mutex                 //防止多协程同时建立连接
	GrpcConnectItemMap map[string]GrpcConnectItem //一开始是空
	consulUrl          string
}

func NewGrpcConnectManager(consulUrl string) *GrpcConnectManager {
	g := &GrpcConnectManager{
		GrpcConnectItemMap: make(map[string]GrpcConnectItem),
		consulUrl:          consulUrl,
	}

	return g
}

//param是指url的参数，例如：wait=10s&tag=mop，代表这个连接超时10秒建立
func (g *GrpcConnectManager) GetConnWithConsul(server, param string) (*grpc.ClientConn, error) {
	if connItem, ok := g.GrpcConnectItemMap[server]; ok { //first check
		if atomic.LoadPointer(&connItem.ClientConn) != nil {
			return (*grpc.ClientConn)(connItem.ClientConn), nil
		}
	}
	g.CreateLock.Lock()
	defer g.CreateLock.Unlock()

	connItem, ok := g.GrpcConnectItemMap[server]
	if ok { //double check
		if atomic.LoadPointer(&connItem.ClientConn) != nil {
			cc := (*grpc.ClientConn)(connItem.ClientConn)
			if g.checkState(cc) == nil {
				return cc, nil
			} else { //旧的连接存在服务端断开的情况，需要先关闭
				cc.Close()
			}
		}
	}

	target := "consul://" + g.consulUrl + "/" + server
	if param != "" {
		target += "?" + param
	}

	cli, err := g.newGrpcConn(target)
	if err != nil {
		return nil, err
	}

	var newItem GrpcConnectItem
	newItem.ServerName = server

	atomic.StorePointer(&newItem.ClientConn, unsafe.Pointer(cli))
	g.GrpcConnectItemMap[server] = newItem

	return cli, nil
}

func (g *GrpcConnectManager) GetConn(server, url string) (*grpc.ClientConn, error) {
	if connItem, ok := g.GrpcConnectItemMap[server+url]; ok { //first check
		if atomic.LoadPointer(&connItem.ClientConn) != nil {
			return (*grpc.ClientConn)(connItem.ClientConn), nil
		}
	}

	g.CreateLock.Lock()
	defer g.CreateLock.Unlock()

	connItem, ok := g.GrpcConnectItemMap[server+url]
	if ok { //double check
		if atomic.LoadPointer(&connItem.ClientConn) != nil {
			cc := (*grpc.ClientConn)(connItem.ClientConn)
			if g.checkState(cc) == nil {
				return cc, nil
			} else { //旧的连接存在服务端断开的情况，需要先关闭
				cc.Close()
			}
		}
	}

	cli, err := g.newGrpcConn(url)
	if err != nil {
		return nil, err
	}

	var newItem GrpcConnectItem
	newItem.ServerName = server

	atomic.StorePointer(&newItem.ClientConn, unsafe.Pointer(cli))
	g.GrpcConnectItemMap[server+url] = newItem

	return cli, nil
}

func (g *GrpcConnectManager) checkState(conn *grpc.ClientConn) error {
	state := conn.GetState()
	switch state {
	case connectivity.TransientFailure, connectivity.Shutdown:
		return errors.New("ErrConn")
	}

	return nil
}

func (g *GrpcConnectManager) newGrpcConn(target string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		target,
		//grpc.WithBlock(),
		grpc.WithInsecure(),
		grpc.WithBalancerName("round_robin"),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(64*1024*1024)),
		//grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
	)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
