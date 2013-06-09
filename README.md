megacmd
=======

A command-line client for mega.co.nz storage service
This utility is written on top of [go-mega](http://github.com/t3rm1n4l/go-mega)

### What is megacmd ?
Mega (mega.co.nz) is an excellent free storage service which provides 50 GB of free storage space. It has a web based user interface to upload and download files.
Megacmd is a command-line tool for performing file and directory transfer between local directories and mega service. Features of megacmd are much similar to s3cmd utility, which is used to perform file transfer to Amazon S3.

### Features
  - Ability to access files and folders in a directory path access URIs
  - Configuration file (~/.megacmd.json)
  - Individual file put and get operations
  - List operation with recursive mode (shows filesize and timestamp)
  - Delete operation on directories and files (soft-delete to trash and hard delete)
  - Move operation to rename and move files or directories
  - Mkdir operation to create directories recursively (Similar to mkdir -p)
  - Sync operation to copy directories recursively between local directory and mega service in both directions
  - Configurable parallel split connections for download and upload to improve transfer speed
  - Download and upload progress bar

### Usage
    Usage ./megacmd:
        megacmd [OPTIONS] list mega:/foo/bar/
        megacmd [OPTIONS] get mega:/foo/file.txt /tmp/
        megacmd [OPTIONS] put /tmp/hello.txt mega:/bar/
        megacmd [OPTIONS] delete mega:/foo/bar
        megacmd [OPTIONS] mkdir mega:/foo/bar
        megacmd [OPTIONS] move mega:/foo/file.txt mega:/bar/foo.txt
        megacmd [OPTIONS] sync mega:/foo/ /tmp/foo/
        megacmd [OPTIONS] sync /tmp/foo mega:/foo

      -conf="/Users/slakshman/.megacmd.json": Config file path
      -force=false: Force hard delete or overwrite
      -help=false: Help
      -recursive=false: Recursive listing
      -verbose=1: Verbose
      -version=false: Version

### How to obtain megacmd ?

#### Compile from source

    $ make
    $ cp megacmd /usr/local/bin

#### Binaries

[Mac OSX](https://mega.co.nz/#!PR9FSKpQ!ez8HoC-LS4m-hBMPGo2K-jZahYFX6dGG65ReyCKKjk)

[Linux](https://mega.co.nz/#!PR9FSKpQ!ez8HoC-LS4m-hBMPGo2K-jZahYFX6dGG65ReyCKKjkE)


### Pitfalls
To list directory contents, use:

    $ megacmd list mega:/foo/bar/

The directory path should end with a suffix '/' to list its contents. Otherwise, it will show metadata for that particular directory or file.

To list trash, use trash:/ root prefix instead of mega:/

To recursively list a directory use, -recursive option.

    $ megacmd -recursive list mega:/foo/bar/

To delete file or directory to trash use, regular delete option as follows:

    $ megacmd delete mega:/foo/bar/file

To delete a file or folder permanently without moving to trash, use -force option:

    $ megacmd -force delete mega:/foo/folder

If you sync command, it will try to copy files to the destination if corresponding files are not present at the destination. It will not overwrite any files if present. It exits by displaying an error message. We can provide -force option with sync command to go forward by overwriting files.


### Sample config file

Create a file ~/.megacmd.json with following content.

    {
        "User" : "MEGA_USERNAME",
        "Password" : "MEGA_PASSWORD",
        "DownloadWorkers" : 4,
        "UploadWorkers" : 4,
        "Verbose" : 1
    }

DownloadWorkers and UploadWorkers specifies how many parallel connections should be used by megacmd.


You can add extra parameters as follows to make the default behavior as follows:
    "Force" : true
    "Recursive" : true

### Examples

    $ megacmd list mega:/
    mega:/gomega-084959515                             314573     2013-06-09T00:23:18+05:30
    mega:/newname.txt                                  31         2013-06-09T00:23:25+05:30
    mega:/dir-1-8cO26/                                 0          2013-06-09T00:23:34+05:30
    mega:/testing/                                     0          2013-06-09T15:14:46+05:30

    $ megacmd put megacmd mega:/testing/
    Copying megacmd -> mega:/testing/ # 100.00 % of 5.6MB at 127K/s 43s 
    Successfully uploaded file megacmd to mega:/testing/ in 44s

    $ megacmd -recursive list mega:/testing/ 
    mega:/testing/x.1                                  51200      2013-06-09T16:03:40+05:30
    mega:/testing/x.2                                  102400     2013-06-09T16:03:48+05:30
    mega:/testing/x.3                                  512000     2013-06-09T16:04:01+05:30
    mega:/testing/x.4                                  1024000    2013-06-09T16:04:40+05:30
    mega:/testing/megacmd                              5553076    2013-06-09T16:18:32+05:30

    $ megacmd -force get mega:/testing/megacmd /tmp/
    Copying mega:/testing/megacmd -> /tmp/megacmd # 100.00 % of 5.6MB at 214K/s 25s 
    Successfully downloaded file mega:/testing/megacmd to /tmp/ in 25s

    $ megacmd move mega:/testing/megacmd mega:/renamedfile
    Successfully moved mega:/testing/megacmd to mega:/renamedfile

    $ megacmd mkdir mega:/dir1/dir2/dir3/dir4
    Successfully created directory at mega:/dir1/dir2/dir3/dir4

    $ megacmd sync mega:/testing /tmp/dir1/
    Found 4 file(s) to be copied
    Copying mega:/testing/x.1 -> /tmp/dir1/x.1 # 100.00 % of 51KB at 1.7K/s 29s 
    Copying mega:/testing/x.2 -> /tmp/dir1/x.2 # 100.00 % of 102KB at 9.2K/s 11s 
    Copying mega:/testing/x.3 -> /tmp/dir1/x.3 # 100.00 % of 512KB at 7.7K/s 1m6s 
    Copying mega:/testing/x.4 -> /tmp/dir1/x.4 # 100.00 % of 1.0MB at 228K/s 4s 
    Successfully sync mega:/testing to /tmp/dir1/ in 1m51s

    $ megacmd delete mega:/testing/x.1
    Successfully deleted  mega:/testing/x.1

### License

MIT
