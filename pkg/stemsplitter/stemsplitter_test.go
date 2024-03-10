package stemsplitter

import (
	"context"
	"os"
	"testing"

	"github.com/docker/docker/client"
)

func TestPullSpleeterImage(t *testing.T) {
	ctx := context.Background()

	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		dockerHost = client.DefaultDockerHost
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}

	err = PullSpleeterImage(ctx, cli)
	if err != nil {
		t.Fatalf("Failed to pull Spleeter image: %v", err)
	}
}
