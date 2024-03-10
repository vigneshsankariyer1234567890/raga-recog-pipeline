package stemsplitter

// TODO: FIGURE OUT INPUTS AND OUTPUTS WHEN RUNNING AND ADD TESTS
import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func PullSpleeterImage(ctx context.Context, cli *client.Client) error {
	spleeterImage := "deezer/spleeter"
	reader, err := cli.ImagePull(ctx, spleeterImage, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	return nil
}

func RunStemSplitting(ctx context.Context, cli *client.Client, audioIn, audioOut, modelDirectory string) error {
	spleeterImage := "deezer/spleeter"
	containerConfig := &container.Config{
		Image: spleeterImage,
		Env:   []string{fmt.Sprintf("MODEL_PATH=%s", modelDirectory)},
		Cmd:   []string{"separate", "-i", fmt.Sprintf("/input/%s", filepath.Base(audioIn)), "-o", "/output", "-p", "spleeter:2stems"},
	}
	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/input", filepath.Dir(audioIn)),
			fmt.Sprintf("%s:/output", audioOut),
			fmt.Sprintf("%s:/model", modelDirectory),
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
