import { describe, it, expect, beforeEach } from 'vitest';
import authReducer, { setCredentials, setPermissions, clearCredentials } from '../authSlice';

beforeEach(() => {
  localStorage.clear();
});

describe('authSlice', () => {
  const initialState = {
    token: null,
    user: null,
    isAuthenticated: false,
  };

  describe('setCredentials', () => {
    it('sets token, user, and isAuthenticated', () => {
      const user = { id: 1, username: 'admin', role: 'super_admin', permissions: [] };
      const state = authReducer(initialState, setCredentials({ token: 'tok_123', user }));
      expect(state.token).toBe('tok_123');
      expect(state.user).toEqual(user);
      expect(state.isAuthenticated).toBe(true);
    });

    it('persists to localStorage', () => {
      const user = { id: 1, username: 'admin', role: 'super_admin' };
      authReducer(initialState, setCredentials({ token: 'tok_123', user }));
      expect(localStorage.getItem('netmonitor_token')).toBe('tok_123');
      expect(JSON.parse(localStorage.getItem('netmonitor_user')!)).toEqual(user);
    });
  });

  describe('setPermissions', () => {
    it('sets permissions on existing user', () => {
      const prevState = {
        token: 'tok',
        user: { id: 1, username: 'admin', role: 'admin' },
        isAuthenticated: true,
      };
      const state = authReducer(prevState, setPermissions(['devices.read', 'alerts.read']));
      expect(state.user!.permissions).toEqual(['devices.read', 'alerts.read']);
    });

    it('persists updated user to localStorage', () => {
      const prevState = {
        token: 'tok',
        user: { id: 1, username: 'admin', role: 'admin' },
        isAuthenticated: true,
      };
      authReducer(prevState, setPermissions(['devices.read']));
      const stored = JSON.parse(localStorage.getItem('netmonitor_user')!);
      expect(stored.permissions).toEqual(['devices.read']);
    });

    it('does nothing when user is null', () => {
      const state = authReducer(initialState, setPermissions(['devices.read']));
      expect(state.user).toBeNull();
    });
  });

  describe('clearCredentials', () => {
    it('clears all auth state', () => {
      const prevState = {
        token: 'tok',
        user: { id: 1, username: 'admin', role: 'admin' },
        isAuthenticated: true,
      };
      const state = authReducer(prevState, clearCredentials());
      expect(state.token).toBeNull();
      expect(state.user).toBeNull();
      expect(state.isAuthenticated).toBe(false);
    });

    it('removes from localStorage', () => {
      localStorage.setItem('netmonitor_token', 'tok');
      localStorage.setItem('netmonitor_user', '{"id":1}');
      authReducer(initialState, clearCredentials());
      expect(localStorage.getItem('netmonitor_token')).toBeNull();
      expect(localStorage.getItem('netmonitor_user')).toBeNull();
    });
  });
});
