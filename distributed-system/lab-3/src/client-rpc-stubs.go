package main

import (
	"context"
	"fmt"

	pb "chord/protocol" // Update path as needed

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// PingNode sends a ping to another node
func PingNode(ctx context.Context, address string) error {
	address = resolveAddress(address)
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChordClient(conn)
	_, err = client.Ping(ctx, &pb.PingRequest{})
	if err != nil {
		return fmt.Errorf("ping failed: %v", err)
	}

	return nil
}

// PutKeyValue sets a key-value pair on a node
func PutKeyValue(ctx context.Context, key, value, address string) error {
	address = resolveAddress(address)
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChordClient(conn)
	_, err = client.Put(ctx, &pb.PutRequest{
		Key:   key,
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("put failed: %v", err)
	}

	return nil
}

// GetValue retrieves a value for a key from a node
func GetValue(ctx context.Context, key, address string) (string, error) {
	address = resolveAddress(address)
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChordClient(conn)
	resp, err := client.Get(ctx, &pb.GetRequest{
		Key: key,
	})
	if err != nil {
		return "", fmt.Errorf("get failed: %v", err)
	}

	return resp.Value, nil
}

// DeleteKey deletes a key from a node
func DeleteKey(ctx context.Context, key, address string) error {
	address = resolveAddress(address)
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChordClient(conn)
	_, err = client.Delete(ctx, &pb.DeleteRequest{
		Key: key,
	})
	if err != nil {
		return fmt.Errorf("delete failed: %v", err)
	}

	return nil
}

// GetAllKeyValues retrieves all key-value pairs from a node
func GetAllKeyValues(ctx context.Context, address string) (map[string]string, error) {
	address = resolveAddress(address)
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChordClient(conn)
	resp, err := client.GetAll(ctx, &pb.GetAllRequest{})
	if err != nil {
		return nil, fmt.Errorf("getall failed: %v", err)
	}

	return resp.KeyValues, nil
}
