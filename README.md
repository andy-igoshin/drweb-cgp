[![English](https://img.shields.io/badge/lang-en-blue.svg)](#) [![Russian](https://img.shields.io/badge/lang-ru-lightgrey.svg)](README.ru.md)

## DrWeb helper for CommuniGate Pro 6.x, *version 2.1.0*

### Introduction

**drweb-cgp** is a *Helper* (external message filter) used to scan email messages for viruses. It receives messages from *CommuniGate Pro* and passes them to *DrWeb* for processing.<br/><br/>

### Features

* The *Helper* receives messages from *CommuniGate Pro* via the *External Filter Interface* protocol and forwards them to *DrWeb* using the *CLAMD* protocol.

* The *Helper* interacts with *DrWeb* using a UNIX socket or TCP. When using a UNIX socket, a file descriptor passing mechanism (FILDES) is utilized; this eliminates data copying between the helper and DrWeb processes.

> Important: When using a UNIX socket, ensure that the user account under which DrWeb is running has read permissions for the CommuniGate Pro queue files.

* The *Helper* instructs *CommuniGate Pro* to discard a message if a virus is detected.

* The *Helper* can forward a copy of the infected message to specified email addresses.

* When forwarding a copy of an infected message, the *Helper* adds an *X-DrWeb-Seen:* header to the processed message to avoid double processing or message looping.

### Installation

> Below are the *Helper* and *Rule* settings in the *CommuniGate Pro* interface.

##### Settings -> General -> Helpers
![](./helper.png)

##### Settings -> Mail -> Rules -> DRWEB
![](./rule.png)


#### Command Line Parameters

```
    Usage of drweb-cgp:
      -debug
            Export debug information (for developers)
      -host string
            DrWeb host to connect (default "/run/.com.drweb.clamd")
      -mirror-to string
            Mirror selected messages to email
      -timeout duration
            DrWeb request timeout (default 3m0s)
      -version
            Print version and exit
```

**debug**

In -debug mode, the helper saves copies of problematic files (those that could not be read or processed) in the *CommuniGate Pro* *BadFiles* directory for further analysis by the developer.

**host**

Path to the UNIX socket or a network address (IP or hostname). If the port is not specified, *3310* is used.

**mirror-to**

A comma-separated list of email addresses where copies of infected messages will be forwarded.

**timeout**

The processing timeout for a single message in *DrWeb*.

</br>

### License

BSD License, [LICENSE.md](LICENSE.md)<br/><br/>

### Author

Andrey Igoshin <<ai@vsu.ru>><br/><br/>

### Links

* Repository: <https://git.vsu.ru/ai/drweb-cgp>
* CommuniGate Pro Website: <https://www.communigate.com>
* Helper Protocol: <https://www.communigate.com/cpro/development/Helpers.html#Filters>
* DrWeb Website: <https://www.drweb.com/>
