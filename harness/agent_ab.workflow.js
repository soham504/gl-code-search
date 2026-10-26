export const meta = {
  name: 'agent-localization-ab',
  description: 'Run 60 localization agents (20 SWE-bench Lite issues x grep / graphify / gl toolkits) through a logged tool harness',
  phases: [{ title: 'Localize', detail: '60 agent runs, one per issue x toolkit' }],
}

// ROOT was the local checkout path in the runs reported in docs/findings.md; pass your clone's path as args.root.
const ROOT = args.root
const TOOLS = {
  grep: `- issue                          print the issue report
- ls [DIR]                       list a directory
- find NAME_GLOB                 find files by name, e.g. find '*admin*.py'
- grep [-i] [-w] [-l] REGEX [SUBPATH]   regex search over .py files (-l: file names only)
- read PATH [START] [END]        print lines of a file (150 lines per call max)`,
}
TOOLS.graphify = TOOLS.grep + `
- graphify report                the Graphify knowledge-graph report for this repo (hubs, communities)
- graphify query "QUESTION"      BFS over the code knowledge graph for a question (add --budget N to raise the token cap)
- graphify explain NAME          a node (function/class/file) and its neighbours
- graphify path A B              shortest path between two nodes
- graphify affected NAME         nodes impacted by NAME (reverse traversal)`
TOOLS.gl = TOOLS.grep + `
- gl locate                      rank the files most likely to need changes for this issue (uses the issue text)
- gl search QUERY WORDS          ranked files with their best-matching lines
- gl symbol NAME                 where NAME is defined (def/class)`

const SCHEMA = {
  type: 'object',
  properties: {
    files: { type: 'array', items: { type: 'string' }, description: 'Up to 5 repo-relative .py paths, most likely first' },
    functions: { type: 'array', items: { type: 'string' }, description: 'Functions/methods to edit, as path::name' },
    confidence: { type: 'string', enum: ['high', 'medium', 'low'] },
  },
  required: ['files', 'functions', 'confidence'],
}

const prompt = (id, cond) => `You are a coding agent doing bug LOCALIZATION: find which source file(s) must be edited to fix a GitHub issue in a Python repository. Do not fix anything.

You can reach the repository ONLY through this command (run it with the Bash tool, from any directory):
  python3 ${ROOT}/harness/tool.py ${id}__${cond} <TOOL> [ARGS...]

Available TOOLs:
${TOOLS[cond]}

Rules (strict — this is a controlled experiment and every call is logged):
- Do not use Read, Grep, Glob, or any other shell command to look at the repository or ANY file in ${ROOT}. Only the command above.
- Do not try to recall or look up the actual upstream fix; reason from the code you see.
- Start with: python3 ${ROOT}/harness/tool.py ${id}__${cond} issue
- Be efficient, like a real engineer: stop once you are confident. Hard limit: 30 tool calls.
- Paths are relative to the repository root. Test files are never the answer.

Return the files that must change (most likely first, max 5), the functions to edit, and your confidence.`

const IDS = args.ids
const runs = []
for (const id of IDS) for (const cond of ['grep', 'graphify', 'gl']) runs.push({ id, cond })

phase('Localize')
const results = await parallel(runs.map(r => () =>
  agent(prompt(r.id, r.cond), { label: `${r.cond}:${r.id}`, phase: 'Localize', schema: SCHEMA })
    .then(out => ({ ...r, out }))))
const done = results.filter(Boolean).filter(r => r.out)
log(`${done.length}/${runs.length} runs returned`)
return done