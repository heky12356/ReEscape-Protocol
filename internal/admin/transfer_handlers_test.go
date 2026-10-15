package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/storage"
)

type adminTransferReader func([]byte) (int, error)

func (reader adminTransferReader) Read(buffer []byte) (int, error) { return reader(buffer) }

func TestTransferAPIReturnsConflictForAnotherActiveJob(t *testing.T) {
	p, _ := setupStateAdmin(t, "sqlite")
	admin := &server{persistence: p}
	engine := admin.routes()
	manager, err := admin.transferManager()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Close)
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		manager.StartImport(adminTransferReader(func([]byte) (int, error) { close(entered); <-release; return 0, io.EOF }))
	}()
	<-entered
	defer func() { close(release); <-finished }()
	for _, path := range []string{"/api/admin/transfers/export", "/api/admin/persistence/backup"} {
		response := stateRequest(engine, "POST", path, true)
		if response.Code != 409 || !strings.Contains(response.Body.String(), "transfer_busy") {
			t.Fatalf("conflict: %d %s", response.Code, response.Body)
		}
	}
}

func TestTransferOverviewOriginAndOfflineRestoreBoundary(t *testing.T) {
	p, _ := setupStateAdmin(t, "file")
	if err := p.Snapshots.Save("state.json", []byte(`{"original":true}`)); err != nil {
		t.Fatal(err)
	}
	admin := &server{persistence: p}
	engine := admin.routes()
	manager, err := admin.transferManager()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Close)
	if r := stateRequest(engine, "GET", "/api/admin/transfers", false); r.Code != 401 {
		t.Fatal(r.Code)
	}
	r := stateRequest(engine, "GET", "/api/admin/transfers", true)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"backend":"file"`) || strings.Contains(r.Body.String(), p.Status().DataDir) {
		t.Fatalf("overview: %d %s", r.Code, r.Body)
	}
	cfg := config.GetConfig()
	previous := cfg.AdminCORSOrigins
	cfg.AdminCORSOrigins = []string{"https://admin.test"}
	t.Cleanup(func() { cfg.AdminCORSOrigins = previous })
	token, _, err := admin.sessions.create(time.Now(), "query-test-key")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/api/admin/transfers/export", nil)
	request.AddCookie(&http.Cookie{Name: adminSessionCookie, Value: token})
	request.Header.Set("Origin", "https://foreign.test")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != 403 {
		t.Fatalf("foreign export: %d", response.Code)
	}
	request = httptest.NewRequest("POST", "/api/admin/transfers/export", nil)
	request.AddCookie(&http.Cookie{Name: adminSessionCookie, Value: token})
	request.Header.Set("Origin", "https://admin.test")
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != 202 {
		t.Fatalf("same origin export: %d %s", response.Code, response.Body)
	}
	var job storage.TransferJob
	if err := json.Unmarshal(response.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	job = awaitMaintenanceJob(t, engine, job.ID)
	if job.Stage != "completed" || job.Report == nil || job.Report.FileCount < 1 || !job.DownloadAvailable {
		t.Fatalf("report: %+v", job)
	}
	for _, path := range []string{"/api/admin/transfers/" + job.ID, "/api/admin/transfers/" + job.ID + "/download"} {
		if r := stateRequest(engine, "GET", path, false); r.Code != 401 {
			t.Fatal(r.Code)
		}
	}
	refused := stateRequest(engine, "POST", "/api/admin/transfers/"+job.ID+"/restore", true)
	if refused.Code != 409 || !strings.Contains(refused.Body.String(), "offline_restore_required") || !strings.Contains(refused.Body.String(), `"applied":false`) {
		t.Fatalf("restore boundary: %d %s", refused.Code, refused.Body)
	}
	data, err := p.Snapshots.Load("state.json")
	if err != nil || string(data) != `{"original":true}` {
		t.Fatal("online restore changed active state")
	}
	if r := stateRequest(engine, "GET", "/api/admin/transfers/missing", true); r.Code != 404 {
		t.Fatal(r.Code)
	}
	// A fresh manager intentionally has no old process job records.
	fresh, err := storage.NewTransferManager(p.Status().DataDir, p.Status().DatabasePath, p.Status().BackupDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if _, ok := fresh.Get(job.ID); ok {
		t.Fatal("job unexpectedly survived new manager")
	}
	if fresh.Overview().InstanceID == manager.Overview().InstanceID {
		t.Fatal("same manager identity after restart")
	}
}

func TestTransferManagerInitializesOnceForConcurrentAdminReads(t *testing.T) {
	p, _ := setupStateAdmin(t, "file")
	admin := &server{persistence: p}
	var workers sync.WaitGroup
	managers := make(chan *storage.TransferManager, 12)
	failures := make(chan error, 12)
	for i := 0; i < 12; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			manager, err := admin.transferManager()
			managers <- manager
			failures <- err
		}()
	}
	workers.Wait()
	close(managers)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first *storage.TransferManager
	for manager := range managers {
		if first == nil {
			first = manager
			t.Cleanup(first.Close)
		} else if manager != first {
			t.Fatal("duplicate transfer managers")
		}
	}
}
