package auth

import (
	"context"
	"sync"
	"time"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// fakeStore keeps the users and sessions in memory, safe for the purge
// goroutine. When err is set, every call fails with it.
type fakeStore struct {
	mu       sync.Mutex
	gm       storage.User
	links    map[string]storage.User // by link hash
	sessions map[string]*fakeSession // by token hash
	err      error
}

type fakeSession struct {
	user      storage.User
	expiresIn time.Duration
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		gm:       storage.User{ID: 1, Name: "mj", Role: storage.RoleGM},
		links:    make(map[string]storage.User),
		sessions: make(map[string]*fakeSession),
	}
}

func (s *fakeStore) GM(context.Context) (storage.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gm, s.err
}

func (s *fakeStore) PlayerByLink(_ context.Context, linkHash []byte) (storage.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return storage.User{}, s.err
	}
	player, ok := s.links[string(linkHash)]
	if !ok {
		return storage.User{}, storage.ErrNotFound
	}
	return player, nil
}

func (s *fakeStore) CreateSession(_ context.Context, tokenHash []byte, userID int, lifetime time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	user := s.gm
	for _, player := range s.links {
		if player.ID == userID {
			user = player
		}
	}
	s.sessions[string(tokenHash)] = &fakeSession{user: user, expiresIn: lifetime}
	return nil
}

func (s *fakeStore) DeleteSession(_ context.Context, tokenHash []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	delete(s.sessions, string(tokenHash))
	return nil
}

func (s *fakeStore) PurgeSessions(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var purged int64
	for hash, session := range s.sessions {
		if session.expiresIn <= 0 {
			delete(s.sessions, hash)
			purged++
		}
	}
	return purged, s.err
}

func (s *fakeStore) sessionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

func (s *fakeStore) expireSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range s.sessions {
		session.expiresIn = -time.Second
	}
}
