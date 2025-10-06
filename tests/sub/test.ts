import { assertEquals } from "jsr:@std/assert"
import { sub } from "./code.ts"

Deno.test("#sub subtracts number", () => {
	assertEquals(sub(2, 1), 1)
})
