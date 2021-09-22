# About pkg

## Fioconv pkg

This pkg convert json result from FIO in CSV table

Use this function to convert

```go
func ConvertJSONtoCSV(inputFile, outputFile string)
```

## Mkconfig pkg

This pkg generate config for FIO util

Use this function to generate:

```go
func GenerateFIOConfig(fType []OperationType, fBS []BlockSize, fJobs []JobsType, fDepth []DepthType, fTime time.Duration, outPath, SshUser string)
```

## SSHwork pkg

The package provides functionality for working with ssh and transferring files via scp

Use this functions for work:

```go
func SendCommandSSH(conn *ssh.Client, command string, foreground bool)
func SendFileSCP(conn *ssh.Client, localPath, remotePath string)
func GetFileSCP(conn *ssh.Client, localPath, remotePath string)
```

## Fiotests

The package provides functionality for running FIO tests via ssh client

Use this functions for work:

```go
func RunFIOTest(sshHost, sshUser, localResultsDir string, fioOptions mkconfig.FioOptions, fioTestTime time.Duration)
```
