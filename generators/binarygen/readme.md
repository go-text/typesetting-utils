# binarygen, a Golang code generator for Opentype files 

This tool extends [go/packages] to understand the syntax describing
the binary layout used is Opentype font files, and generates Go parsing functions.
 
## Custom syntax 

The binary layout is specified in Go source files using struct tags :

- 'arrayCount' : FirstUint16 | FirstUint32 | ToEnd | To-<XXX> | ComputedField-<XXX>
- 'offsetSize' : Offset16 | Offset32
- 'offsetsArray' : Offset16 | Offset32 , for an array of offsets. Zero offsets are resolved to zero values.
- 'offsetRelativeTo' : Parent | GrandParent 
- 'unionField' : the name of a previous field 
- 'unionTag' : the value of the tag identifying an union member
- 'isOpaque' : anything (even the empty string), to use custom parsing/writing functions
- 'subsliceStart' : AtStart | AtCurrent , used for opaque fields and raw data ([]byte)
- 'arguments' : a comma separated list of values to pass to the field parsing function

The special comment `// binarygen: argument=<name> <type>` indicates that the parsing function requires additionnal argument.