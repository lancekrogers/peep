package media

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os/exec"
)

const SpectrumBands = 24
const sampleRate = 16000
const spectrumWindow = 512

type SpectrumFrame struct {
	Time   float64
	Levels [SpectrumBands]float64
	Err    error
}

// Spectrum analyzes actual mono PCM in log-spaced bands, with bounded prefetch.
// Its timestamps share the media clock used by mpv and are reset on seeks.
func Spectrum(ctx context.Context, path string, start float64) <-chan SpectrumFrame {
	out := make(chan SpectrumFrame, 2)
	go func() {
		defer close(out)
		send := func(f SpectrumFrame) bool {
			select {
			case out <- f:
				return true
			case <-ctx.Done():
				return false
			}
		}
		args := []string{"-v", "error", "-nostdin", "-threads", "1"}
		if start > 0 {
			args = append(args, "-ss", fmt.Sprintf("%.3f", start))
		}
		args = append(args, "-i", path, "-vn", "-sn", "-ac", "1", "-ar", "16000", "-f", "s16le", "pipe:1")
		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			send(SpectrumFrame{Err: err})
			return
		}
		if err = cmd.Start(); err != nil {
			send(SpectrumFrame{Err: err})
			return
		}
		buf := make([]byte, spectrumWindow*2)
		samples := make([]float64, spectrumWindow)
		count := 0
		for {
			n, readErr := io.ReadFull(pipe, buf)
			if n == 0 {
				break
			}
			for i := range samples {
				samples[i] = 0
				if 2*i+1 < n {
					samples[i] = float64(int16(binary.LittleEndian.Uint16(buf[i*2:]))) / 32768
				}
			}
			f := SpectrumFrame{Time: start + float64(count*spectrumWindow)/sampleRate, Levels: analyzeSpectrum(samples)}
			if !send(f) {
				break
			}
			count++
			if readErr != nil {
				break
			}
		}
		err = cmd.Wait()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			send(SpectrumFrame{Err: fmt.Errorf("audio spectrum: %w", err)})
		}
	}()
	return out
}
func analyzeSpectrum(samples []float64) (levels [SpectrumBands]float64) {
	n := len(samples)
	if n < 2 {
		return
	}
	for band := range levels {
		frequency := 60 * math.Pow(7000.0/60, float64(band)/(SpectrumBands-1))
		coefficient := 2 * math.Cos(2*math.Pi*frequency/sampleRate)
		s1, s2 := 0.0, 0.0
		for i, v := range samples {
			window := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n-1))
			s := v*window + coefficient*s1 - s2
			s2 = s1
			s1 = s
		}
		power := max(0, s1*s1+s2*s2-coefficient*s1*s2)
		amplitude := 4 * math.Sqrt(power) / float64(n)
		db := 20 * math.Log10(max(amplitude, 1e-9))
		levels[band] = max(0, min(1, (db+60)/60))
	}
	return
}
