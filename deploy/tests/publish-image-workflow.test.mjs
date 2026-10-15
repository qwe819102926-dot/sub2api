import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const workflow = readFileSync(new URL('../../.github/workflows/publish-image.yml', import.meta.url), 'utf8')
const scriptBlock = workflow.match(/^ {10}script: \|\r?\n((?: {12}[^\r\n]*(?:\r?\n|$)|\r?\n)+)/m)
assert.ok(scriptBlock, 'The promotion gate must contain a GitHub script')
const script = scriptBlock[1].replace(/^ {12}/gm, '')
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor
const verify = new AsyncFunction('github', 'context', 'core', script)
const sha = 'f'.repeat(40)

async function runGate(runs) {
  const outputs = {}
  const failures = []
  const calls = []
  const summary = {
    addHeading() { return this },
    addRaw() { return this },
    addLink() { return this },
    async write() {},
  }
  await verify(
    { rest: { actions: { async listWorkflowRuns(options) {
      calls.push(options)
      return { data: { workflow_runs: runs } }
    } } } },
    { repo: { owner: 'owner', repo: 'repo' }, sha },
    { setFailed: message => failures.push(message), setOutput: (key, value) => { outputs[key] = value }, info() {}, summary },
  )
  assert.deepEqual(calls, [{ owner: 'owner', repo: 'repo', workflow_id: 'smoke-test.yml', branch: 'dev', head_sha: sha, status: 'success', per_page: 10 }])
  return { outputs, failures }
}

const successfulRun = { id: 123, head_sha: sha, head_branch: 'dev', event: 'push', conclusion: 'success', html_url: 'https://github.com/owner/repo/actions/runs/123' }

for (const event of ['push', 'workflow_dispatch']) {
  test(`promotes the exact successful dev ${event} run`, async () => {
    const result = await runGate([{ ...successfulRun, event }])
    assert.deepEqual(result.outputs, { 'run-id': 123 })
    assert.deepEqual(result.failures, [])
  })
}

for (const [name, fields] of [
  ['another commit', { head_sha: 'a'.repeat(40) }],
  ['another branch', { head_branch: 'main' }],
  ['a pull request', { event: 'pull_request' }],
  ['a workflow run event', { event: 'workflow_run' }],
  ['a failed run', { conclusion: 'failure' }],
  ['an unfinished run', { conclusion: null }],
]) {
  test(`rejects ${name}`, async () => {
    const result = await runGate([{ ...successfulRun, ...fields }])
    assert.deepEqual(result.outputs, {})
    assert.equal(result.failures.length, 1)
  })
}

test('rejects a commit without a successful dev run', async () => {
  const result = await runGate([])
  assert.deepEqual(result.outputs, {})
  assert.equal(result.failures.length, 1)
})

test('selects a valid manual run after unrelated successful runs', async () => {
  const result = await runGate([
    { ...successfulRun, id: 1, head_sha: 'a'.repeat(40) },
    { ...successfulRun, id: 2, head_branch: 'main' },
    { ...successfulRun, id: 3, event: 'pull_request' },
    { ...successfulRun, event: 'workflow_dispatch' },
  ])
  assert.deepEqual(result.outputs, { 'run-id': 123 })
  assert.deepEqual(result.failures, [])
})
