package audiosegmenter

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"sync"

	ffmpeg_go "github.com/u2takey/ffmpeg-go"
)

type FormatInfo struct {
	Duration string `json:"duration"`
}

type FFProbeOutput struct {
	Format FormatInfo `json:"format"`
}

func ParseDuration(probeOutput string) (float64, error) {
	var probe FFProbeOutput
	err := json.Unmarshal([]byte(probeOutput), &probe)

	if err != nil {
		return 0, fmt.Errorf("failed to unmarshal probe output: %w", err)
	}

	duration, err := strconv.ParseFloat(probe.Format.Duration, 64)

	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}

func CopyAudioSegment(inputFilePath string, segmentIdx int, segmentStart int, segmentDuration int, outputDir string) error {
	filename := filepath.Base(inputFilePath)
	filenameWithoutExt := filename[:len(filename)-len(filepath.Ext(filename))]
	outputFilePath := fmt.Sprintf("%s/%s_seg_%d.mp3", outputDir, filenameWithoutExt, segmentIdx)

	err := ffmpeg_go.Input(inputFilePath).Output(outputFilePath, ffmpeg_go.KwArgs{
		"ss": strconv.Itoa(segmentStart),
		"t":  strconv.Itoa(segmentDuration),
		"c":  "copy",
		"y":  "",
	}).Run()

	if err != nil {
		return fmt.Errorf("failed to create segment %d for %s: %w", segmentIdx, outputFilePath, err)
	}

	return nil
}

func SegmentAudio(inputFilePath string, segmentDuration int, outputDir string) []error {
	probeOutput, err := ffmpeg_go.Probe(inputFilePath)

	if err != nil {
		return []error{err}
	}

	duration, err := ParseDuration(probeOutput)

	if err != nil {
		return []error{err}
	}

	numberOfSegments := int(math.Ceil(duration / float64(segmentDuration)))

	errChan := make(chan error, numberOfSegments)

	var wg sync.WaitGroup

	for i := 0; i < numberOfSegments; i++ {
		wg.Add(1)

		go func(segmentIdx int) {
			defer wg.Done()
			err := CopyAudioSegment(inputFilePath, segmentIdx, segmentIdx*segmentDuration, segmentDuration, outputDir)

			if err != nil {
				errChan <- fmt.Errorf("error processing segment %d: %w", segmentIdx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for e := range errChan {
		errs = append(errs, e)
	}

	return errs
}
