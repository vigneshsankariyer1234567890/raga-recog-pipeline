package audiosegmenter

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ffmpeg_go "github.com/u2takey/ffmpeg-go"
)

const INPUT_DIR = "./sample/kiravani"
const OUTPUT_DIR = "./output"
const FILE_NAME = "01-kaligiyuNTEgadA_galgunu-kIravANi.mp3"

func GET_EXPECTED_OUTPUT_FILE_NAME(segmentIdx int) string {
	return fmt.Sprintf("01-kaligiyuNTEgadA_galgunu-kIravANi_seg_%d.mp3", segmentIdx)
}

type ParseDurationTest struct {
	name         string
	probeOutput  string
	wantDuration float64
	wantErr      bool
}

func TestParseDuration(t *testing.T) {
	tests := []ParseDurationTest{
		{
			name:         "valid duration",
			probeOutput:  `{"format": {"duration": "123.45"}}`,
			wantDuration: 123.45,
			wantErr:      false,
		},
		{
			name:         "invalid JSON",
			probeOutput:  `{"format": {"duration":}}`,
			wantDuration: 0,
			wantErr:      true,
		},
		{
			name:         "invalid duration format",
			probeOutput:  `{"format": {"duration": "abc"}}`,
			wantDuration: 0,
			wantErr:      true,
		},
	}

	for _, parseTest := range tests {
		t.Run(parseTest.name, func(t *testing.T) {
			duration, err := ParseDuration(parseTest.probeOutput)
			if (err != nil) != parseTest.wantErr {
				t.Errorf("ParseDuration() error = %v, wantErr %v", err, parseTest.wantErr)
				return
			}
			if duration != parseTest.wantDuration {
				t.Errorf("ParseDuration() = %v, want %v", duration, parseTest.wantDuration)
			}
		})
	}
}

func TestCopyAudioSegment_Positive(t *testing.T) {
	inputDir := INPUT_DIR
	outputDir := OUTPUT_DIR
	segmentDuration := 30

	os.MkdirAll(outputDir, 0755)
	defer os.RemoveAll(outputDir)

	inputFilePath := filepath.Join(inputDir, FILE_NAME)
	segmentIdx := 0
	segmentStart := segmentIdx * segmentDuration

	err := CopyAudioSegment(inputFilePath, segmentIdx, segmentStart, segmentDuration, outputDir)
	if err != nil {
		t.Fatalf("Failed to copy audio segment: %v", err)
	}

	expectedOutputFileName := filepath.Join(outputDir, GET_EXPECTED_OUTPUT_FILE_NAME(segmentIdx))

	if _, err := os.Stat(expectedOutputFileName); os.IsNotExist(err) {
		t.Fatalf("Output file does not exist: %s", expectedOutputFileName)
	}

	probeOutput, err := ffmpeg_go.Probe(expectedOutputFileName)
	if err != nil {
		t.Fatalf("Failed to probe output file: %v", err)
	}

	duration, err := ParseDuration(probeOutput)
	if err != nil {
		t.Fatalf("Failed to parse duration from probe output: %v", err)
	}

	if duration > float64(segmentDuration)+1 || duration < float64(segmentDuration)-1 {
		t.Errorf("Segment duration is incorrect: got %v seconds, want around %v seconds", duration, segmentDuration)
	}
}

func TestCopyAudioSegment_LastSegment(t *testing.T) {
	inputDir := INPUT_DIR
	outputDir := OUTPUT_DIR

	err := os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}
	defer os.RemoveAll(outputDir)

	inputFilePath := filepath.Join(inputDir, FILE_NAME)

	probeOutput, err := ffmpeg_go.Probe(inputFilePath)
	if err != nil {
		t.Fatalf("Failed to probe input file: %v", err)
	}

	duration, err := ParseDuration(probeOutput)
	if err != nil {
		t.Fatalf("Failed to parse duration from probe output: %v", err)
	}

	segmentDuration := 30
	numberOfSegments := int(duration) / segmentDuration
	lastSegmentStart := numberOfSegments * segmentDuration

	expectedLastSegmentDuration := int(duration) - lastSegmentStart
	if expectedLastSegmentDuration > segmentDuration {
		expectedLastSegmentDuration = segmentDuration
	}

	err = CopyAudioSegment(inputFilePath, numberOfSegments, lastSegmentStart, segmentDuration, outputDir)
	if err != nil {
		t.Fatalf("Failed to copy last audio segment: %v", err)
	}

	expectedOutputFileName := filepath.Join(outputDir, GET_EXPECTED_OUTPUT_FILE_NAME(numberOfSegments))

	if _, err := os.Stat(expectedOutputFileName); os.IsNotExist(err) {
		t.Fatalf("Output file does not exist: %s", expectedOutputFileName)
	}

	probeOutputForOutput, errOutput := ffmpeg_go.Probe(expectedOutputFileName)
	if errOutput != nil {
		t.Fatalf("Failed to probe output file: %v", errOutput)
	}

	durationOutput, err := ParseDuration(probeOutputForOutput)
	if err != nil {
		t.Fatalf("Failed to parse duration from probe output: %v", err)
	}

	if durationOutput > float64(expectedLastSegmentDuration)+1 || durationOutput < float64(expectedLastSegmentDuration)-1 {
		t.Errorf("Segment duration is incorrect: got %v seconds, want around %v seconds", durationOutput, expectedLastSegmentDuration)
	}
}

func TestCopyAudioSegment_NegativeCases(t *testing.T) {
	// Negative Test Case 1: Invalid input file
	{
		outputDir := OUTPUT_DIR
		os.MkdirAll(outputDir, 0755)
		defer os.RemoveAll(outputDir) // Clean up after test

		invalidInputFilePath := "nonexistent.mp3"
		err := CopyAudioSegment(invalidInputFilePath, 0, 0, 30, outputDir)
		if err == nil {
			t.Errorf("Expected error for non-existent input file, but no error was returned")
		}
	}
}

func TestSegmentAudio(t *testing.T) {
	inputDir := INPUT_DIR
	outputBaseDir := OUTPUT_DIR
	const segmentDuration = 30

	inputFilePath := filepath.Join(inputDir, FILE_NAME)
	outputDir := filepath.Join(outputBaseDir, "kiravani", fileNameWithoutExtension(FILE_NAME))

	err := os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		t.Fatalf("Failed to create base output directory: %v", err)
	}
	// defer os.RemoveAll(outputBaseDir)

	errors := SegmentAudio(inputFilePath, segmentDuration, outputDir)

	if len(errors) > 0 {
		for _, err := range errors {
			t.Errorf("Error while segmenting audio: %v", err)
		}
	}

	probeOutput, _ := ffmpeg_go.Probe(inputFilePath)
	duration, _ := ParseDuration(probeOutput)
	expectedSegments := int(math.Ceil(duration / float64(segmentDuration)))

	files, _ := os.ReadDir(outputDir)
	if len(files) != expectedSegments {
		t.Errorf("Expected %d segments, found %d", expectedSegments, len(files))
	}

	for i, file := range files {
		segmentPath := filepath.Join(outputDir, file.Name())
		segmentOutput, _ := ffmpeg_go.Probe(segmentPath)
		segmentDuration, _ := ParseDuration(segmentOutput)

		if i < expectedSegments-1 && segmentDuration != float64(segmentDuration) {
			t.Errorf("Segment %d duration incorrect: got %v seconds, want %v seconds", i, segmentDuration, segmentDuration)
		}
		if i == expectedSegments-1 && segmentDuration > float64(segmentDuration) {
			t.Errorf("Last segment duration too long: got %v seconds, want <= %v seconds", segmentDuration, segmentDuration)
		}
	}
}

func fileNameWithoutExtension(fp string) string {
	return strings.TrimSuffix(filepath.Base(fp), filepath.Ext(fp))
}
