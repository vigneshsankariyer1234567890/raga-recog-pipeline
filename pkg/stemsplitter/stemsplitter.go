package stemsplitter

// TODO: FIGURE OUT INPUTS AND OUTPUTS WHEN RUNNING AND ADD TESTS
import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

func PullDockerImage(ctx context.Context, cli *client.Client, dockerImage string) error {
	images, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list Docker images: %w", err)
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == dockerImage {
				return nil
			}
		}
	}

	reader, err := cli.ImagePull(ctx, dockerImage, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	if err != nil {
		return fmt.Errorf("failed to read the output of image pull: %w", err)
	}

	_, err = io.Copy(io.Discard, reader) // Use io.Discard if you don't want the output
	if err != nil {
		return fmt.Errorf("failed to read the output of image pull: %w", err)
	}

	return nil
}

func CreateModelVolume(ctx context.Context, cli *client.Client, volumeName string) error {
	_, err := cli.VolumeCreate(ctx, volume.CreateOptions{Name: volumeName})
	if err != nil {
		return err
	}
	return nil
}

func EnsureModelVolumeExists(ctx context.Context, cli *client.Client, volumeName string) error {
	volumeList, err := cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return err
	}
	for _, vol := range volumeList.Volumes {
		if vol.Name == volumeName {
			return nil
		}
	}

	return CreateModelVolume(ctx, cli, volumeName)
}

func RunStemSplitting(ctx context.Context, cli *client.Client, audioIn, audioOut, modelVolumeName, spleeterImage string) error {
	containerConfig := &container.Config{
		Image: spleeterImage,
		Env:   []string{"MODEL_PATH=/model"},
		Cmd:   []string{"separate", "-i", fmt.Sprintf("/input/%s", filepath.Base(audioIn)), "-o", "/output", "-p", "spleeter:2stems"},
	}
	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/input", filepath.Dir(audioIn)),
			fmt.Sprintf("%s:/output", audioOut),
			fmt.Sprintf("%s:/model", modelVolumeName),
		},
	}

	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return err
	}
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-statusCh:
		// Container finished
	}

	return nil
}
