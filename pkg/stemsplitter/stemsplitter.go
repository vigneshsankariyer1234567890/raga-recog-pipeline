package stemsplitter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
)

const MAX_ALLOWABLE_CONCURRENCY = 10

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

	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to read the output of image pull: %w", err)
	}

	return nil
}

func BuildImage(ctx context.Context, cli *client.Client, dockerfilePath, contextPath, imageName string) error {
	dockerfileRelativePath := filepath.Base(dockerfilePath)
	contextDir, _ := filepath.Abs(contextPath)
	tar, err := archive.TarWithOptions(contextDir, &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("failed to tar context directory: %v", err)
	}
	defer tar.Close()

	buildOptions := types.ImageBuildOptions{
		Tags:       []string{imageName},
		Dockerfile: dockerfileRelativePath,
		Remove:     true, // Remove intermediate containers after a successful build
	}

	buildResponse, err := cli.ImageBuild(ctx, tar, buildOptions)
	if err != nil {
		return fmt.Errorf("failed to build Docker image: %v", err)
	}
	defer buildResponse.Body.Close()

	if err != nil {
		return fmt.Errorf("failed to read the output of image build: %w", err)
	}

	decoder := json.NewDecoder(buildResponse.Body)

	for {
		var message jsonmessage.JSONMessage

		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				fmt.Println("EOF reached, breaking out of the loop.")
				break
			}
			return fmt.Errorf("error reading JSON message: %v", err)
		}

		if message.Error != nil {
			return fmt.Errorf("error from daemon while building: %v", message.Error.Message) // Corrected to message.Error.Message
		} else if message.Stream != "" {
			fmt.Print(message.Stream)
		}

		if message.Progress != nil {
			fmt.Print(message.Progress)
		} else {
			fmt.Println("Received a message without stream or progress:", message)
		}
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

func getEnv(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}

func moveFiles(sourceDir, targetDir string) []error {
	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
		return []error{fmt.Errorf("failed to create target directory: %v", err)}
	}

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return []error{fmt.Errorf("failed to read source directory: %v", err)}
	}

	actualConcurrency := min(MAX_ALLOWABLE_CONCURRENCY, len(entries))

	var wg sync.WaitGroup
	sem := make(chan struct{}, actualConcurrency)
	errChan := make(chan error, len(entries))

	for _, entry := range entries {
		wg.Add(1)
		go func(entry os.DirEntry) {
			defer wg.Done()
			sem <- struct{}{}

			defer func() { <-sem }()

			info, err := entry.Info()

			if err != nil {
				errChan <- fmt.Errorf("failed to get info for %s: %v", entry.Name(), err)
				return
			}

			if !info.IsDir() {
				srcPath := filepath.Join(sourceDir, entry.Name())
				dstPath := filepath.Join(targetDir, entry.Name())

				if err := os.Rename(srcPath, dstPath); err != nil {
					errChan <- fmt.Errorf("failed to move file from %s to %s: %v", srcPath, dstPath, err)
				}
			}
		}(entry)
	}

	wg.Wait()

	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func RunStemSplitting(ctx context.Context, cli *client.Client, audioIn, audioOut, modelVolumePath, demucsImage string) error {
	absAudioIn, err := filepath.Abs(audioIn)
	if err != nil {
		return fmt.Errorf("error getting absolute path for audio input: %v", err)
	}
	absAudioOut, err := filepath.Abs(audioOut)
	if err != nil {
		return fmt.Errorf("error getting absolute path for audio output: %v", err)
	}
	absModelVolumePath, err := filepath.Abs(modelVolumePath)
	if err != nil {
		return fmt.Errorf("error getting absolute path for model volume: %v", err)
	}
	envVars := []string{
		fmt.Sprintf("GPU=%s", getEnv("GPU", "false")),
		fmt.Sprintf("MP3OUTPUT=%s", getEnv("MP3OUTPUT", "true")),
		fmt.Sprintf("MODEL=%s", getEnv("MODEL", "htdemucs")),
	}

	containerConfig := &container.Config{
		Image: demucsImage,
		Env:   envVars,
		Cmd:   []string{filepath.Base(absAudioIn)},
	}

	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/data/input", filepath.Dir(absAudioIn)),
			fmt.Sprintf("%s:/data/output", absAudioOut),
			fmt.Sprintf("%s:/data/models", absModelVolumePath),
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

	dirWithoutExt := strings.TrimSuffix(filepath.Base(absAudioIn), filepath.Ext(absAudioIn))
	destPath := fmt.Sprintf("%s/%s", absAudioOut, dirWithoutExt)
	currSource := fmt.Sprintf("%s/htdemucs/%s", absAudioOut, dirWithoutExt)
	defer os.RemoveAll(currSource)

	errs := moveFiles(currSource, destPath)

	if errs != nil {
		var errMsgs []string
		for _, err := range errs {
			if err != nil {
				errMsgs = append(errMsgs, err.Error())
			}
		}
		return fmt.Errorf("errors occured while moving files: %s", strings.Join(errMsgs, "; "))
	}

	return nil
}
