import { createSlice, type PayloadAction } from '@reduxjs/toolkit';

interface AuthState {
  token: string | null;
  user: { id: number; username: string; role: string } | null;
  isAuthenticated: boolean;
}

const initialState: AuthState = {
  token: localStorage.getItem('netmonitor_token'),
  user: (() => {
    try { return JSON.parse(localStorage.getItem('netmonitor_user') || 'null'); } catch { return null; }
  })(),
  isAuthenticated: !!localStorage.getItem('netmonitor_token'),
};

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    setCredentials(state, action: PayloadAction<{ token: string; user: { id: number; username: string; role: string } }>) {
      state.token = action.payload.token;
      state.user = action.payload.user;
      state.isAuthenticated = true;
      localStorage.setItem('netmonitor_token', action.payload.token);
      localStorage.setItem('netmonitor_user', JSON.stringify(action.payload.user));
    },
    clearCredentials(state) {
      state.token = null;
      state.user = null;
      state.isAuthenticated = false;
      localStorage.removeItem('netmonitor_token');
      localStorage.removeItem('netmonitor_user');
    },
  },
});

export const { setCredentials, clearCredentials } = authSlice.actions;
export default authSlice.reducer;
