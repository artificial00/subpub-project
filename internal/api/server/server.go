package server

import (
	"context"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"subpub-project/internal/proto"
	"subpub-project/pkg/subpub"
)

type Server struct {
	proto.UnimplementedPubSubServer
	engine subpub.SubPub
}

func New(engine subpub.SubPub) *Server {
	return &Server{engine: engine}
}

func (s *Server) Subscribe(req *proto.SubscribeRequest, stream proto.PubSub_SubscribeServer) error {
	if req.GetKey() == "" {
		return status.Error(codes.InvalidArgument, "key cannot be empty")
	}

	sub, err := s.engine.Subscribe(req.GetKey(), func(msg interface{}) {
		str, ok := msg.(string)
		if !ok {
			log.Error().Msg("invalid message type")
			return
		}

		if err := stream.Send(&proto.Event{Data: str}); err != nil {
			log.Error().Err(err).Msg("failed to send message to stream")
		}
	})
	if err != nil {
		return status.Errorf(codes.Internal, "subscribe failed: %v", err)
	}

	<-stream.Context().Done()
	sub.Unsubscribe()
	return nil
}

func (s *Server) Publish(ctx context.Context, req *proto.PublishRequest) (*emptypb.Empty, error) {
	if req.GetKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "key cannot be empty")
	}

	if err := s.engine.Publish(req.GetKey(), req.GetData()); err != nil {
		return nil, status.Errorf(codes.Internal, "publish failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}
