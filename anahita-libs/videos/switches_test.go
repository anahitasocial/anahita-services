package videos

import "testing"

// Audio posts are on wherever videos are, unless said off; recording is
// off unless said on, and never where audio posts are off.
func TestTheAudioSwitches(t *testing.T) {
	cases := []struct {
		video, audio, recording string
		uploads, records        bool
	}{
		{"", "", "on", false, false},
		{"on", "", "", true, false},
		{"on", "", "on", true, true},
		{"on", "off", "on", false, false},
		{"on", "OFF", "", false, false},
	}
	for _, each := range cases {
		t.Setenv(EnvUploads, each.video)
		t.Setenv(EnvAudioUploads, each.audio)
		t.Setenv(EnvAudioRecording, each.recording)
		if AudioUploadsOn() != each.uploads || AudioRecordingOn() != each.records {
			t.Errorf("%+v: uploads %v, recording %v", each, AudioUploadsOn(), AudioRecordingOn())
		}
	}
}
