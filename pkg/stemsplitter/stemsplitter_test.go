package stemsplitter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

const SPLEETER_IMAGE_TO_PULL = "deezer/spleeter:3.8-5stems"
const SPLEETER_TEST_VOLUME_NAME = "spleeter-models-test"

func checkDockerDaemon() error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("Docker daemon is not running or did not respond within time limit: %w", err)
	}

	fmt.Println("Docker daemon is running.")
	return nil
}

func TestPullSpleeterImage(t *testing.T) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}

	doneCh := make(chan error, 1)

	go func() {
		err = PullDockerImage(ctx, cli, SPLEETER_IMAGE_TO_PULL)
		doneCh <- err
	}()

	testTimeout := 2 * time.Minute

	select {
	case <-time.After(testTimeout):
		t.Fatal("Test timed out")
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("Failed to pull Spleeter image: %v", err)
		}
	}

	images, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		t.Fatalf("Failed to list Docker images: %v", err)
	}

	found := false
	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == SPLEETER_IMAGE_TO_PULL {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("The image %s was not pulled as expected", SPLEETER_IMAGE_TO_PULL)
	}
}

func TestEnsureModelVolumeExists(t *testing.T) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer cli.Close()

	defer func() {
		_ = cli.VolumeRemove(ctx, SPLEETER_TEST_VOLUME_NAME, true)
	}()

	if err := EnsureModelVolumeExists(ctx, cli, SPLEETER_TEST_VOLUME_NAME); err != nil {
		t.Errorf("Failed to ensure model volume exists on first call: %v", err)
	}

	if err := EnsureModelVolumeExists(ctx, cli, SPLEETER_TEST_VOLUME_NAME); err != nil {
		t.Errorf("Failed to ensure model volume exists on second call: %v", err)
	}

	volumeList, err := cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list Docker volumes: %v", err)
	}
	found := false
	for _, vol := range volumeList.Volumes {
		if vol.Name == SPLEETER_TEST_VOLUME_NAME {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Test volume %s was not created as expected", SPLEETER_TEST_VOLUME_NAME)
	}
}

func TestRunStemSplittingSpecificFile(t *testing.T) {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer cli.Close()

	if err := EnsureModelVolumeExists(ctx, cli, SPLEETER_TEST_VOLUME_NAME); err != nil {
		t.Fatalf("Failed to ensure model volume exists: %v", err)
	}

	audioInPath, err := filepath.Abs("../../sample/kiravani/01-kaligiyuNTEgadA_galgunu-kIravANi_seg_10.mp3")
	if err != nil {
		t.Fatalf("Failed to get absolute path for audio input: %v", err)
	}

	audioOutPath, err := filepath.Abs("../../output/kiravani/01-kaligiyuNTEgadA_galgunu-kIravANi/01-kaligiyuNTEgadA_galgunu-kIravANi_seg_10")
	if err != nil {
		t.Fatalf("Failed to get absolute path for audio output: %v", err)
	}

	// Run stem splitting
	if err := RunStemSplitting(ctx, cli, audioInPath, audioOutPath, SPLEETER_TEST_VOLUME_NAME, SPLEETER_IMAGE_TO_PULL); err != nil {
		t.Fatalf("Failed to run stem splitting: %v", err)
	}

	// Verification steps to ensure stem splitting ran as expected
	// This might include checking if the expected output files exist, etc.
}

func TestMain(m *testing.M) {
	if err := checkDockerDaemon(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	exitVal := m.Run()

	os.Exit(exitVal)
}
