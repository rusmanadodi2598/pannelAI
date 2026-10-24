// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_agnes_test.go
// @for       The Agnes video dialect: the task id its create answers, the poll URL that id rebuilds, and the two answer shapes.
// @uses      testing, internal/dataplane, internal/schema.
// @reason    The provider's poll needs the video id and the model together, and the client carries only one task id, so the encoding is the seam that makes keyframe and reference jobs pollable; these tests pin both directions and the URL it builds.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-21
package service

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

func TestAgnesTaskID_RoundTrips(t *testing.T) {
	cases := []struct {
		name        string
		taskID      string
		wantVideoID string
		wantModel   string
	}{
		{
			name:        "the encoded id carries the video and its model",
			taskID:      "dmlkZW9fMXxhZ25lcy12aWRlby0yLjUtZmxhc2g",
			wantVideoID: "video_1",
			wantModel:   "agnes-video-2.5-flash",
		},
		{
			name:        "a raw provider id is read as the video id",
			taskID:      "video_1",
			wantVideoID: "video_1",
		},
		{
			name:        "an encoded id without a model keeps the video alone",
			taskID:      "dmlkZW9fMXw",
			wantVideoID: "video_1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			videoID, model := splitAgnesTaskID(tc.taskID)
			if videoID != tc.wantVideoID || model != tc.wantModel {
				t.Fatalf("splitAgnesTaskID(%q) = %q, %q; want %q, %q", tc.taskID, videoID, model, tc.wantVideoID, tc.wantModel)
			}
		})
	}
	if got := agnesTaskID("video_1", "agnes-video-2.5-flash"); got != "dmlkZW9fMXxhZ25lcy12aWRlby0yLjUtZmxhc2g" {
		t.Fatalf("agnesTaskID() = %q", got)
	}
}

func TestBuildAgnesVideoPoll(t *testing.T) {
	call := MediaCall{BaseURL: "https://apihub.agnes-ai.com/v1/videos"}
	cases := []struct {
		name    string
		taskID  string
		wantURL string
	}{
		{
			name:    "the encoded id rebuilds both parameters",
			taskID:  agnesTaskID("video_1", "agnes-video-2.5-flash"),
			wantURL: "https://apihub.agnes-ai.com/agnesapi?model_name=agnes-video-2.5-flash&video_id=video_1",
		},
		{
			name:    "a raw provider id omits the model the text mode does not need",
			taskID:  "video_1",
			wantURL: "https://apihub.agnes-ai.com/agnesapi?video_id=video_1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request, err := buildAgnesVideoPoll(call, tc.taskID, nil)
			if err != nil {
				t.Fatalf("buildAgnesVideoPoll() error = %v", err)
			}
			if request.Method != "GET" || request.URL != tc.wantURL {
				t.Fatalf("request = %s %s, want GET %s", request.Method, request.URL, tc.wantURL)
			}
		})
	}
	if _, err := buildAgnesVideoPoll(call, "  ", nil); err == nil {
		t.Fatal("buildAgnesVideoPoll() must refuse an empty task id")
	}
}

func TestReadAgnesVideoJob(t *testing.T) {
	t.Run("a creation encodes the retrieval id", func(t *testing.T) {
		body := `{"id":"task_1","task_id":"task_1","video_id":"video_1",` +
			`"model":"agnes-video-2.5-flash","status":"queued","progress":0}`
		job, err := readAgnesVideoJob([]byte(body), true)
		if err != nil {
			t.Fatalf("readAgnesVideoJob() error = %v", err)
		}
		if job.TaskID != agnesTaskID("video_1", "agnes-video-2.5-flash") {
			t.Fatalf("task id = %q, want the retrieval id and model encoded", job.TaskID)
		}
		if job.Status != schema.VideoJobQueued || job.Progress == nil || *job.Progress != 0 {
			t.Fatalf("job = %+v, want a queued job at 0 percent", job)
		}
	})

	t.Run("a poll reports the completed asset", func(t *testing.T) {
		body := `{"id":"task_1","status":"completed","progress":100,` +
			`"url":"https://cdn.agnes/v.mp4","error":null}`
		job, err := readAgnesVideoJob([]byte(body), false)
		if err != nil {
			t.Fatalf("readAgnesVideoJob() error = %v", err)
		}
		if job.Status != schema.VideoJobSucceeded || job.URL != "https://cdn.agnes/v.mp4" {
			t.Fatalf("job = %+v, want the succeeded asset", job)
		}
		if job.TaskID != "" {
			t.Fatalf("task id = %q, want none: the poll request already carries it", job.TaskID)
		}
	})

	t.Run("a failed poll carries the provider's message", func(t *testing.T) {
		body := `{"id":"task_1","status":"failed","error":{"message":"the prompt was rejected"}}`
		job, err := readAgnesVideoJob([]byte(body), false)
		if err != nil {
			t.Fatalf("readAgnesVideoJob() error = %v", err)
		}
		if job.Status != schema.VideoJobFailed || job.Error != "the prompt was rejected" {
			t.Fatalf("job = %+v, want the failed job and its message", job)
		}
	})

	t.Run("a creation that names no job is refused", func(t *testing.T) {
		if _, err := readAgnesVideoJob([]byte(`{"status":"queued"}`), true); err == nil {
			t.Fatal("readAgnesVideoJob() must refuse a creation without a job id")
		}
	})

	t.Run("a poll without a state is refused", func(t *testing.T) {
		if _, err := readAgnesVideoJob([]byte(`{"id":"task_1"}`), false); err == nil {
			t.Fatal("readAgnesVideoJob() must refuse a poll without a state")
		}
	})
}

// TestAgnesVideoPlanIsRegistered pins that the provider reaches the builders
// this file tests rather than the xAI default.
func TestAgnesVideoPlanIsRegistered(t *testing.T) {
	plan := videoPlanFor("AGnes")
	if plan.create != nil || plan.createPath != "" {
		t.Fatal("agnes creates at the collection root; a create path or builder would move it")
	}
	if plan.poll == nil || plan.read == nil {
		t.Fatal("agnes needs its own poll builder and reader")
	}
}
