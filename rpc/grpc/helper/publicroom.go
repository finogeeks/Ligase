package helper

import (
	"github.com/finogeeks/ligase/model/publicroomstypes"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
)

func ToPublicRoomState(v *publicroomstypes.PublicRoom) *pb.PublicRoomState {
	return &pb.PublicRoomState{
		RoomID:           v.RoomID,
		Aliases:          v.Aliases,
		CanonicalAlias:   v.CanonicalAlias,
		Name:             v.Name,
		Topic:            v.Topic,
		AvatarURL:        v.AvatarURL,
		NumJoinedMembers: v.NumJoinedMembers,
		WorldReadable:    v.WorldReadable,
		GuestCanJoin:     v.GuestCanJoin,
	}
}

func ToPublicRoom(v *pb.PublicRoomState) *publicroomstypes.PublicRoom {
	return &publicroomstypes.PublicRoom{
		RoomID:           v.RoomID,
		Aliases:          v.Aliases,
		CanonicalAlias:   v.CanonicalAlias,
		Name:             v.Name,
		Topic:            v.Topic,
		AvatarURL:        v.AvatarURL,
		NumJoinedMembers: v.NumJoinedMembers,
		WorldReadable:    v.WorldReadable,
		GuestCanJoin:     v.GuestCanJoin,
	}
}
