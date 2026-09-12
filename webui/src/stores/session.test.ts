import { beforeEach, describe, expect, it } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { useSessionStore } from "./session";
describe("web session", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    setActivePinia(createPinia());
  });
  it("keeps only CSRF metadata in session storage", () => {
    const store = useSessionStore();
    store.establish(
      {
        id: 1,
        username: "admin",
        role: "admin",
        enabled: true,
        mustChangePassword: false,
      },
      "csrf",
    );
    expect(store.connected).toBe(true);
    expect(sessionStorage.getItem("ds.csrf")).toBe("csrf");
    expect(localStorage.length).toBe(0);
    store.clear();
    expect(sessionStorage.getItem("ds.csrf")).toBeNull();
  });
});
