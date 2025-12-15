package main

import (
	"context"
	"crypto/tls"
	"fmt"

	pb "chord/protocol" // Update path as needed

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// get TLS option
func getDialOpts() grpc.DialOption {
	// InsecureSkipVerify: true
	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	return grpc.WithTransportCredentials(credentials.NewTLS(config))
}

// PingNode sends a ping to another node
func PingNode(ctx context.Context, address string) error {
	address = resolveAddress(address)
	conn, err := grpc.NewClient(address, getDialOpts())
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
	conn, err := grpc.NewClient(address, getDialOpts())
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
	conn, err := grpc.NewClient(address, getDialOpts())
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
	conn, err := grpc.NewClient(address, getDialOpts())
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
	conn, err := grpc.NewClient(address, getDialOpts())
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

// RPC call (external) for find the closet predecessor of a look-up ID
func CallFindClosetPredecessor(ctx context.Context, remoteAddr string, lookupID string) (string, error) {
	remoteAddr = resolveAddress(remoteAddr)
	conn, err := grpc.NewClient(remoteAddr, getDialOpts())
	if err != nil {
		return "", fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChordClient(conn)
	resp, err := client.FindClosestPreceding(ctx, &pb.FindClosestPrecedingRequest{Id: lookupID})

	if err != nil {
		return "", err
	}

	return resp.Address, nil
}

// RPC call to notify an address that current address might be the predecessor
func CallNotify(address string, currentAddress string) error {
	address = resolveAddress(address)
	conn, err := grpc.NewClient(address, getDialOpts())
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChordClient(conn)
	_, err = client.Notify(context.Background(), &pb.NotifyRequest{
		Address: currentAddress,
	})
	return err
}

// RPC call to get predecessor of a node (definied by its address)
func CallGetPredecessor(address string) (string, error) {
	address = resolveAddress(address)
	conn, err := grpc.NewClient(address, getDialOpts())
	if err != nil {
		return "", fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChordClient(conn)
	resp, err := client.GetPredecessor(context.Background(), &pb.GetPredecessorRequest{})
	if err != nil {
		return "", err
	}
	return resp.Address, nil
}

// Get a successor list of a node (indicated by its address), essential for the maintenace
func CallGetSuccessorList(address string) ([]string, error) {
	address = resolveAddress(address)
	conn, err := grpc.NewClient(address, getDialOpts())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewChordClient(conn)
	resp, err := client.GetSuccessorList(context.Background(), &pb.GetSuccessorListRequest{})

	if err != nil {
		return nil, err
	}

	if len(resp.Successors) == 0 {
		return nil, fmt.Errorf("chord %s returned an empty successors list", address)
	}

	return resp.Successors, nil
}

// Get a successor of a node (indicated by address)
// Put data to replica
func CallPutReplica(address, key, value string) error {
	address = resolveAddress(address)
	conn, err := grpc.NewClient(address, getDialOpts())
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewChordClient(conn)
	_, err = client.Put(context.Background(), &pb.PutRequest{
		Key:       key,
		Value:     value,
		IsReplica: true,
	})
	return err
}
