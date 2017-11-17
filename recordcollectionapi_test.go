package main

import (
	"context"
	"testing"
)

func InitTestServer() *Server {
	return &Server{}
}

func TestGetRecords(t *testing.T) {
	s := InitTestServer()
	_, err := s.GetRecords(context.Background(), nil)

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}
}
