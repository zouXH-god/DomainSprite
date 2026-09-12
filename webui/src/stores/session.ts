import { defineStore } from "pinia";
import { ref } from "vue";
export interface LoginUser {
  id: number;
  username: string;
  role: "admin" | "user";
  enabled: boolean;
  mustChangePassword: boolean;
}
export const useSessionStore = defineStore("session", () => {
  const user = ref<LoginUser>();
  const csrfToken = ref(sessionStorage.getItem("ds.csrf") || "");
  const connected = ref(false);
  function establish(next: LoginUser, csrf = "") {
    user.value = next;
    connected.value = true;
    csrfToken.value = csrf;
    if (csrf) sessionStorage.setItem("ds.csrf", csrf);
  }
  function clear() {
    user.value = undefined;
    connected.value = false;
    csrfToken.value = "";
    sessionStorage.removeItem("ds.csrf");
  }
  return { user, csrfToken, connected, establish, clear };
});
