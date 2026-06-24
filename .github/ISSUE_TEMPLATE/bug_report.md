name: Bug report
description: Report something that does not work the way the README says it should.
labels: ['bug']
body:
  - type: markdown
    attributes:
      value: |
        Thanks for taking the time to file a bug. Please include everything
        the maintainers need to reproduce it.

  - type: input
    id: os
    attributes:
      label: OS
      placeholder: 'Windows 11 23H2, Ubuntu 24.04, macOS 15, ...'
    validations:
      required: true

  - type: input
    id: go-version
    attributes:
      label: Go version (if building from source)
      placeholder: 'go1.26.1'
    validations:
      required: false

  - type: input
    id: binary
    attributes:
      label: Which binary
      placeholder: 'iRUN / iRUN-find / sshr'
    validations:
      required: true

  - type: textarea
    id: what-happened
    attributes:
      label: What happened?
      placeholder: 'Command run, output seen, error message...'
    validations:
      required: true

  - type: textarea
    id: expected
    attributes:
      label: What did you expect to happen?
    validations:
      required: true

  - type: textarea
    id: repro
    attributes:
      label: Steps to reproduce
      placeholder: |
        1. Run `iRUN.exe` on host A
        2. Run `sshr user@A:2222 whoami` on host B
        3. ...
    validations:
      required: true
