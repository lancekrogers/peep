package media

import (
	"context"
	"math"
	"os"
	"testing"
	"time"
)

func TestSpectrumRespondsToAudio(t *testing.T) {
	silent := analyzeSpectrum(make([]float64, spectrumWindow))
	for _, v := range silent {
		if v != 0 {
			t.Fatal("silence animated the spectrum")
		}
	}
	peaks := []int{}
	for _, frequency := range []float64{120, 2000} {
		samples := make([]float64, spectrumWindow)
		for i := range samples {
			samples[i] = 0.7 * math.Sin(2*math.Pi*frequency*float64(i)/sampleRate)
		}
		levels := analyzeSpectrum(samples)
		peak := 0
		for i, v := range levels {
			if v > levels[peak] {
				peak = i
			}
		}
		if levels[peak] < 0.5 {
			t.Fatal("tone produced no significant energy")
		}
		peaks = append(peaks, peak)
	}
	if peaks[0] >= peaks[1] {
		t.Fatalf("frequency did not move spectrum peak: %v", peaks)
	}
}
func TestSpectrumStream(t *testing.T) {
	if os.Getenv("PEEP_INTEGRATION") == "" {
		t.Skip("integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch := Spectrum(ctx, "../../tmp/fixtures/04-audio.wav", 2)
	f := <-ch
	if f.Err != nil {
		t.Fatal(f.Err)
	}
	if f.Time != 2 {
		t.Fatalf("seek timestamp: %v", f.Time)
	}
	energy := 0.0
	for _, v := range f.Levels {
		energy += v
	}
	if energy == 0 {
		t.Fatal("decoded tone has no spectrum")
	}
	cancel()
	for range ch {
	}
}
