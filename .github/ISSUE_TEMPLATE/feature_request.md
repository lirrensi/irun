name: Feature request
description: Propose a change to iRUN. Please open an issue before sending a PR for new behavior.
labels: ['enhancement']
body:
  - type: textarea
    id: problem
    attributes:
      label: Problem
      placeholder: 'What is the user-facing problem this would solve?'
    validations:
      required: true

  - type: textarea
    id: proposal
    attributes:
      label: Proposed solution
      placeholder: 'How should it work? Show commands, flags, or example output.'
    validations:
      required: true

  - type: textarea
    id: alternatives
    attributes:
      label: Alternatives considered
    validations:
      required: false
