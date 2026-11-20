import { request } from "./client.ts";

/** Settings are the settings of the instance, readable without a session. */
export type Settings = {
  languages: {
    source: string;
    con: string;
  };
};

export function getSettings(): Promise<Settings> {
  return request("GET", "/settings");
}
