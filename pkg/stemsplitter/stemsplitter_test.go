package stemsplitter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// "path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

const (
	SPLEETER_IMAGE_TO_PULL    = "deezer/spleeter:3.8-5stems"
	SPLEETER_TEST_VOLUME_NAME = "spleeter-models-test"
	DEMUCS_IMAGE_NAME         = "wrapped-demucs:test"
	DEMUCS_DOCKER_IMG_PATH    = "../../docker/demucs.dockerfile"
	DEMUCS_DOCKER_CONTEXT     = "../../docker"
	TEST_AUDIO_IN             = "../../sample/kiravani/01-kaligiyuNTEgadA_galgunu-kIravANi_seg_30.mp3"
	TEST_AUDIO_OUT            = "../../output/kiravani/01-kaligiyuNTEgadA_galgunu-kIravANi"
	TEST_MODEL_VOLUME_DIR     = "../../models"
)

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

func TestBuildImage(t *testing.T) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}

	doneCh := make(chan error, 1)
	go func() {
		err = BuildImage(ctx, cli, DEMUCS_DOCKER_IMG_PATH, DEMUCS_DOCKER_CONTEXT, DEMUCS_IMAGE_NAME)
		doneCh <- err
	}()

	testTimeout := 5 * time.Minute // Larger since we actually have to build it from scratch

	select {
	case <-time.After(testTimeout):
		t.Fatal("Test timed out")
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("Failed to list Docker images: %v", err)
		}
	}

	images, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		t.Fatalf("Failed to list Docker images: %v", err)
	}

	found := false
	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == DEMUCS_IMAGE_NAME {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("The image %s was not pulled as expected", DEMUCS_IMAGE_NAME)
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

func TestRunStemSplitting(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}

	if err := BuildImage(ctx, cli, DEMUCS_DOCKER_IMG_PATH, DEMUCS_DOCKER_CONTEXT, DEMUCS_IMAGE_NAME); err != nil {
		t.Fatalf("Failed to build image: %v", err)
	}

	if err := RunStemSplitting(ctx, cli, TEST_AUDIO_IN, TEST_AUDIO_OUT, TEST_MODEL_VOLUME_DIR, DEMUCS_IMAGE_NAME); err != nil {
		t.Fatalf("Failed to run stem splitting: %v", err)
	}

	// Check TEST_AUDIO_OUT/Base(TEST_AUDIO_IN) without suffix exists
	expectedSubDir := strings.TrimSuffix(filepath.Base(TEST_AUDIO_IN), filepath.Ext(TEST_AUDIO_IN))
	expectedOutputDir := fmt.Sprintf("%s/%s", TEST_AUDIO_OUT, expectedSubDir)
	defer os.RemoveAll(expectedOutputDir)

	dirs, err := os.ReadDir(expectedOutputDir)
	if err != nil {
		t.Fatalf("Did not split successfully: %v", err)
	}

	if len(dirs) < 4 {
		t.Fatalf("Did not produce split files as expected, only got %v", len(dirs))
	}
}

func TestMain(m *testing.M) {
	if err := checkDockerDaemon(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	exitVal := m.Run()

	os.Exit(exitVal)
}
