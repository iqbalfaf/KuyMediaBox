package ffmpeg

import (
	"errors"
	"strings"
	"testing"

	"kuymediabox/internal/queue"
)

const sampleProbe = `{
 "streams": [
  {"index":0,"codec_type":"video","codec_name":"hevc","width":2532,"height":1170,"avg_frame_rate":"30000/1001","r_frame_rate":"30/1",
   "side_data_list":[{"rotation":-90}],"disposition":{"attached_pic":0}},
  {"index":1,"codec_type":"audio","codec_name":"aac","sample_rate":"44100","channels":2,"sample_fmt":"fltp","disposition":{"attached_pic":0}},
  {"index":2,"codec_type":"video","codec_name":"mjpeg","width":600,"height":600,"disposition":{"attached_pic":1}}
 ],
 "format": {"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"75.5","bit_rate":"2000000"}
}`

func TestParseProbe(t *testing.T) {
	info, err := parseProbe([]byte(sampleProbe))
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasVideo || !info.HasAudio {
		t.Fatal("streams not detected")
	}
	if info.Width != 1170 || info.Height != 2532 {
		t.Errorf("rotation not applied: %dx%d", info.Width, info.Height)
	}
	if info.Duration != 75.5 {
		t.Errorf("duration %v", info.Duration)
	}
	if info.CoverIndex != 2 || info.CoverCodec != "mjpeg" {
		t.Errorf("cover %d %s", info.CoverIndex, info.CoverCodec)
	}
	if info.FPS < 29.9 || info.FPS > 30 {
		t.Errorf("fps %v", info.FPS)
	}
	if info.BitsPerSample != 24 {
		t.Errorf("float audio should report 24 bit, got %d", info.BitsPerSample)
	}
}

func TestParseProbeNoStreams(t *testing.T) {
	if _, err := parseProbe([]byte(`{"streams":[],"format":{}}`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestFriendly(t *testing.T) {
	err := friendly("[mp4 @ 0x1] Could not find tag for codec vp9 in stream #0, codec not currently supported in container", errors.New("exit 1"))
	var ue *queue.UserError
	if !errors.As(err, &ue) || !strings.Contains(ue.Message, "Codec tidak cocok") {
		t.Fatalf("got %v", err)
	}
	err = friendly("something odd happened\nConversion failed!", errors.New("exit 1"))
	if !errors.As(err, &ue) || !strings.Contains(ue.Message, "something odd happened") {
		t.Fatalf("got %v", err)
	}
}
