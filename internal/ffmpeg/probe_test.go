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

const mkvProbe = `{
 "streams": [
  {"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"avg_frame_rate":"24000/1001"},
  {"index":1,"codec_type":"audio","codec_name":"aac","sample_rate":"48000","channels":2},
  {"index":2,"codec_type":"subtitle","codec_name":"ass","disposition":{"default":1,"forced":0},"tags":{"language":"ind","title":"Indonesia"}},
  {"index":3,"codec_type":"subtitle","codec_name":"subrip","disposition":{"default":0,"forced":1},"tags":{"language":"und"}},
  {"index":4,"codec_type":"attachment","codec_name":"ttf","tags":{"filename":"arial.ttf","mimetype":"application/x-truetype-font"}},
  {"index":5,"codec_type":"attachment","tags":{"filename":"cover.jpg","mimetype":"image/jpeg"}},
  {"index":6,"codec_type":"attachment","codec_name":"otf","tags":{"filename":"Font.OTF","mimetype":"application/octet-stream"}}
 ],
 "format": {"format_name":"matroska,webm","duration":"1400"}
}`

func TestParseProbeSubsAndFonts(t *testing.T) {
	info, err := parseProbe([]byte(mkvProbe))
	if err != nil {
		t.Fatal(err)
	}
	if info.SubCodec != "ass" || len(info.Subs) != 2 {
		t.Fatalf("subs %q %+v", info.SubCodec, info.Subs)
	}
	if s := info.Subs[0]; s.Index != 2 || s.Lang != "ind" || s.Title != "Indonesia" || !s.Default || s.Forced {
		t.Errorf("first sub %+v", s)
	}
	if s := info.Subs[1]; s.Index != 3 || s.Codec != "subrip" || s.Lang != "" || !s.Forced {
		t.Errorf("second sub %+v", s)
	}
	if len(info.Fonts) != 2 || info.Fonts[0].Index != 4 || info.Fonts[1].Name != "Font.OTF" {
		t.Errorf("fonts %+v", info.Fonts)
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
