/**
 * build_app — project AI tool: build StateTrak for a given operating system.
 *
 * Loaded by pi from .pi/extensions/ when working in this repo. Gives the
 * agent a single tool instead of remembering the Makefile targets and the
 * Wails cross-OS limitations:
 *
 *   - target=server : pure-Go cross-compile, works for any os from any host
 *                     (`make release` — all four server binaries)
 *   - target=desktop|dmg + os == the CURRENT host : local Wails build
 *                     (`make desktop` / `make dmg`; the client build is a
 *                     make dependency)
 *   - target=desktop + a DIFFERENT os : Wails needs each OS's webview SDK,
 *                     so it can't be built locally — dispatch that OS's
 *                     release workflow (release-<os>.yml, needs `gh`
 *                     authenticated) and return the run URL; publish=true
 *                     attaches the artifact to the dispatched tag's release
 */

import { spawn } from "node:child_process";
import { cpSync, existsSync, rmSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import { Type } from "@earendil-works/pi-ai";
import { defineTool, type ExtensionAPI } from "@earendil-works/pi-coding-agent";

const HOST_OS = { darwin: "macos", win32: "windows", linux: "linux" }[
	process.platform
] as string;

// wails lives in ~/go/bin which GUI-launched processes may not have on PATH
function wailsBinary(): string {
	const p = join(homedir(), "go", "bin", process.platform === "win32" ? "wails.exe" : "wails");
	return existsSync(p) ? p : "wails";
}

function run(
	cmd: string,
	args: string[],
	opts: { cwd?: string; signal?: AbortSignal } = {},
): Promise<{ code: number; output: string }> {
	return new Promise((resolve, reject) => {
		const child = spawn(cmd, args, {
			cwd: opts.cwd,
			env: process.env,
			stdio: ["ignore", "pipe", "pipe"],
		});
		let output = "";
		child.stdout.on("data", (d: Buffer) => (output += d.toString()));
		child.stderr.on("data", (d: Buffer) => (output += d.toString()));
		opts.signal?.addEventListener("abort", () => child.kill("SIGKILL"), { once: true });
		child.on("error", reject);
		child.on("close", (code) => resolve({ code: code ?? 1, output }));
	});
}

const tail = (s: string, max = 4000) =>
	s.length > max ? "…\n" + s.slice(-max) : s;

const buildApp = defineTool({
	name: "build_app",
	label: "Build StateTrak",
	description:
		"Build the StateTrak app for a given operating system. " +
		"target=desktop builds the Wails desktop app — locally when os matches " +
		`the current host (${HOST_OS}), otherwise it dispatches that OS's release workflow (release-<os>.yml, needs gh authenticated) and returns the run URL. ` +
		"target=dmg builds the macOS disk image (os must be macos). " +
		"target=server cross-compiles the pure-Go server binaries locally for all platforms regardless of os. " +
		"publish=true (CI dispatches only) attaches the artifact to the GitHub release — requires dispatching a tag ref.",
	parameters: Type.Object({
		os: Type.Union([Type.Literal("macos"), Type.Literal("windows"), Type.Literal("linux")], {
			description: "Target operating system",
		}),
		target: Type.Union(
			[Type.Literal("desktop"), Type.Literal("server"), Type.Literal("dmg")],
			{ description: "What to build: the Wails desktop app, its macOS dmg, or the server binaries" },
		),
		publish: Type.Optional(
			Type.Boolean({
				description: "CI dispatch only: also publish the artifact to the dispatched tag's GitHub release (the ref must be a tag)",
			}),
		),
	}),

	async execute(_toolCallId, params, signal, onUpdate, _ctx) {
		const { os, target } = params;
		const steps: string[] = [];
		let result = "";

		if (target === "dmg" && os !== "macos") {
			throw new Error("dmg is macOS-only — use target=desktop for other OSes");
		}

		// ---- server: pure Go, cross-compiles from any host ----
		if (target === "server") {
			onUpdate?.(`cross-compiling server binaries (client + 4 platforms)…`);
			const r = await run("make", ["release"], { signal });
			if (r.code !== 0) throw new Error(tail(r.output));
			steps.push("make release");
			result =
				`Server binaries built into release/ (all four platforms — the requested "${os}" ones included):\n` +
				`  release/statetrak-${os === "macos" ? "macos-arm64 + statetrak-macos-intel" : os + "-amd64"}${os === "windows" ? ".exe" : ""}\n` +
				"These are browser-mode binaries (no window; they auto-open the default browser).";
		}

		// ---- desktop/dmg for the current host: build locally ----
		else if (os === HOST_OS) {
			onUpdate?.(`building ${target} locally on ${HOST_OS} (client + wails, a few minutes)…`);
			let r: { code: number; output: string };
			if (HOST_OS === "macos") {
				r = await run("make", [target === "dmg" ? "dmg" : "desktop"], { signal });
			} else {
				// hosts without make (windows) / linux webkit2gtk-4.1:
				// replicate `make client` in node, then invoke wails directly
				const npm = process.platform === "win32" ? "npm.cmd" : "npm";
				const ci = await run(npm, ["ci"], { cwd: "client", signal });
				if (ci.code !== 0) throw new Error(tail(ci.output));
				const build = await run(npm, ["run", "build"], { cwd: "client", signal });
				if (build.code !== 0) throw new Error(tail(build.output));
				rmSync("web/dist", { recursive: true, force: true });
				cpSync("client/build", "web/dist", { recursive: true });
				writeFileSync("web/dist/.gitkeep", "");
				r = await run(
					wailsBinary(),
					["build", ...(HOST_OS === "linux" ? ["-tags", "webkit2_41"] : [])],
					{ cwd: "desktop", signal },
				);
			}
			if (r.code !== 0) throw new Error(tail(r.output));
			steps.push(HOST_OS === "macos" ? `make ${target === "dmg" ? "dmg" : "desktop"}` : "wails build");
			result =
				HOST_OS === "macos"
					? target === "dmg"
						? "Built release/StateTrak.dmg and desktop/build/bin/StateTrak.app (universal)."
						: "Built desktop/build/bin/StateTrak.app (universal). Add target=dmg for a disk image."
					: `Built desktop/build/bin/StateTrak${process.platform === "win32" ? ".exe" : ""} on the ${HOST_OS} host.`;
		}

		// ---- desktop for another OS: dispatch that OS's release workflow ----
		else {
			const publish = params.publish === true;
			const branch = await run("git", ["rev-parse", "--abbrev-ref", "HEAD"], { signal });
			if (branch.code !== 0) throw new Error("could not determine the git branch: " + branch.output);
			const ref = branch.output.trim();

			const auth = await run("gh", ["auth", "status"], { signal });
			if (auth.code !== 0) {
				throw new Error(
					`gh is not authenticated — run "gh auth login" first (the ${os} desktop app can only be built on ${os} or via GitHub Actions)`,
				);
			}
			onUpdate?.(`dispatching the ${os} release workflow…`);
			const dispatch = await run("gh", [
				"workflow", "run", `release-${os}.yml`,
				"--ref", ref,
				"-f", `publish=${publish}`,
			], { signal });
			if (dispatch.code !== 0) throw new Error(tail(dispatch.output));

			const latest = await run("gh", [
				"run", "list", `--workflow=release-${os}.yml`, "--limit", "1",
				"--json", "url,status", "--jq", ".[0] | \"\\(.status) \\(.url)\"",
			], { signal });
			steps.push(`gh workflow run release-${os}.yml (publish=${publish})`);
			result =
				`Dispatched the ${os} desktop build on GitHub (ref ${ref}). ` +
				`Wails apps need each OS's webview SDK, so cross-desktop builds run in CI.\n` +
				(latest.code === 0 ? `Run: ${latest.output.trim()}\n` : "") +
				(publish
					? "publish=true — the artifact will attach to this ref's release. NOTE: publishing requires the ref to be a version tag; a branch ref builds only."
					: "Build-only run; add publish=true to attach the artifact to a tag's release.") +
				`\nTakes ~5-10 min; the artifact also lands on the run page (dmg for macos, zip for windows, tar.gz for linux).`;
		}

		return {
			content: [{ type: "text", text: result + (steps.length ? `\n\nSteps: ${steps.join(" → ")}` : "") }],
			details: { os, target, hostOS: HOST_OS, steps },
		};
	},
});

export default function (pi: ExtensionAPI) {
	pi.registerTool(buildApp);
}