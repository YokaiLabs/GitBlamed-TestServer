import { assertEquals } from "jsr:@std/assert"
import { sum } from "./code.ts"

Deno.test("#sum adds number", () => {
	assertEquals(sum(1, 2), 3)
})
