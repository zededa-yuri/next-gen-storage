# About pkg

## Fioconv pkg

This pkg convert json result from FIO in CSV table

Use this function to convert

```code
func ConvertJSONtoCSV(inputFile, outputFile string)
```

## Mkconfig pkg

This pkg generate config for FIO util

Use this function to generate:

```code
func GenerateFIOConfig(fType OpType, fBS BlockSize, fJobs JobsType, fDepth DepthType, fTime, outPath string)
```

PublicType:

```code
type OpType []string
type BlockSize []string
type JobsType []int
type DepthType []int
```

## Scpwork pkg

The package provides functionality for working with ssh and transferring files via scp

Use this functions for work:

```code
func SendCommandSSH(ip *string, port *int, user, password, command string, foreground bool)
func SendFileSCP(ip *string, port *int, user, password, filename, destpath string)
func GetFileSCP(ip *string, port *int, user, password, filename, destpath string)
```
