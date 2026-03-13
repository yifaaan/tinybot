// Package cronrepo contains file-backed repositories used by service/cron.
//
// It is responsible for scheduled-job persistence only:
//   - load the current job list from the workspace
//   - validate jobs at the repository boundary
//   - save cron execution state after each trigger pass
//
// It should not decide when to poll, execute the agent, or render CLI output.
package cronrepo
