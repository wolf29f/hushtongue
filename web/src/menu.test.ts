import { expect, test } from "vitest";
import { menuFor } from "./menu.ts";

test("the GM sees the players", () => {
  expect(menuFor("gm").map((entry) => entry.label)).toEqual([
    "Traduction",
    "Dictionnaire",
    "Textes",
    "Joueurs",
  ]);
});

test("a player doesn't see the players", () => {
  expect(menuFor("player").map((entry) => entry.label)).toEqual([
    "Traduction",
    "Dictionnaire",
    "Textes",
  ]);
});
