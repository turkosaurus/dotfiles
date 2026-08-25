---
name: write-tech
description: Use when drafting or rewriting technical prose, including incident summaries, issues, or PR bodies.
---

# write-tech

## tl;dr

Bulleted facts anchored to evidence links. Prose only for what a link can't say.

## principles

- achieve clarity though brevity
- only essential points should be discussed (rely on links if context might be needed)
- whittle down flowery prose until only minimum, essential, and direct verbiage exist
- better to err on the thin side: it's easier to elaborate than to rewrite slop

## rules

- human prose is golden: not only should you never rewrite a human edit (exempting spelling, glaring grammar, etc), ensure that all agent written text aligns to linguistic and formatting norms established by the human
  - only edit human misspellings or glaring grammatical errors
  - always inform a human of the correction
  - never try to quietly improve human prose
  - use 
- no hard line breaks
- no em dashes
- no idioms or metaphors, (exemptions for common jargon)
- all factual claims should be backed with a link to the occurrence (telemetry, action run, etc.)

## format

Use informational sections when helpful, omit otherwise.

### issues

First section should be a single sentence.

#### example

> ## {tl;dr|why|motivation}
>
> Freshdesk may accept an attachment but fail to confirm, causing us to retry and double the attachment.
>
> ## impact
>
> Freshdesk flakiness causes specious error, and client then throws unnecessary error to user. ([slack](URL), [helpdesk-ticket](URL))
> 
> ## details
>
> - service-A made UPDATE to upstream which succeeded, but the server timed out giving response ([event](URL))
> - request was retried because no 20x was received, causing a second duplicate payload to arrive ([event](URL))
> - 3 similar events found in the last 7 days ([event](URL), [event](URL), [event](URL))
>
> ## context
>
> Prior fixes to same symptom were client-side (but this issue is server side):
> - [retry logic](https://github.com/owner/repo-a/blob/main/pkgB/file.go#L5-L15)
> - [config](https://github.com/owner/repo-a/blob/main/pkgA/file.go#L453)
> - https://github.com/owner/repo-a/pull/123
> - https://github.com/owner/repo-b/pull/456
> - https://github.com/owner/repo-b/issue/9876
> 
> ## {deliverable|acceptance|proposal|solution}
> Handle case of successful upload but failed reply 

### PRs

#### example

> ## context
> - closes https://github.com/owner/repo-b/pull/456
> - https://github.com/owner/repo-a/pull/123
> 
> ## summary
> To minimize specious client errors, every potential failure mode has been accounted for, adding additional checks and retries before returning an error.
> 
> - updated [retry logic](https://github.com/owner/repo-a/blob/main/pkgB/file.go#L5-L15) to make GET after potential partial failure, and continue if the file of the correct size already exists
> - {etc}
> 
> ## deploy
> - [ ] verify [build](URL)
> - [ ] monitor SigNoz [deployment dashboard](URL) for errors
> - [ ] confirm `X` behavior has ceased in telemetry
> - [ ] confirm that `Y` behavior is now visible in telemetry (add link to event as comment) 
