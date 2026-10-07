[![English](https://img.shields.io/badge/lang-en-blue.svg)](#) [![Russian](https://img.shields.io/badge/lang-ru-lightgrey.svg)](README.ru.md)

## DrWeb helper for CommuniGate Pro 6.x, *version 2.2.8*

### Introduction

**drweb-cgp** is a *Helper* (external message filter) used to scan email messages for viruses. It receives messages from *CommuniGate Pro* and passes them to *DrWeb* for processing.<br/><br/>

### Features

* The *Helper* receives messages from *CommuniGate Pro* via the *External Filter Interface* protocol and forwards them to *DrWeb* using the *CLAMD* protocol.

* The *Helper* interacts with *DrWeb* using a UNIX socket or TCP. When using a UNIX socket, a file descriptor passing mechanism (FILDES) is utilized; this eliminates data copying between the helper and DrWeb processes.

> Important: When using a UNIX socket, ensure that the user account under which DrWeb is running has read permissions for the CommuniGate Pro queue files.

* The *Helper* instructs *CommuniGate Pro* to discard a message if a virus is detected.

* The Helper can notify the message sender if a virus is detected. Notifications are sent to either authenticated users or external senders whose *Envelope&nbsp;FROM* domain is verified by a *DKIM* signature (details are available in the configuration file).

> *DKIM* verification is performed only upon virus detection and only if notifications are enabled.

* The *Helper* can forward a copy of the infected message to specified email addresses.

* When forwarding a copy of an infected message, the *Helper* adds an *X-DrWeb-Seen:* header to the processed message to avoid double processing or message looping.

### Installation

The *Helper* configuration file [drweb-cgp.yml](drweb-cgp.yml) is located by default in the same directory as the **drweb-cgp** executable. An alternative configuration file path can be specified via the command line. All available settings are described in detail within the configuration file.

> Below are the *Helper* and *Rule* settings in the *CommuniGate Pro* interface.

##### Settings -> General -> Helpers
![](./helper.png)

##### Settings -> Mail -> Rules -> DRWEB
![](./rule.png)


#### Command Line Parameters

```
    Usage of drweb-cgp:
      -config string
            Path to configuration file (default "drweb-cgp.yml")
      -configdump
            Dump configuration and exit
      -configtest
            Test configuration syntax and exit
      -debug
            Enable debug logging
      -version
            Print version and exit
```

**config**

Specifies an alternative configuration file.

**configdump**

Outputs the configuration file in a formatted form.

**configtest**

Checks the configuration file for syntax errors.

**debug**

In -debug mode, the helper saves copies of problematic files (those that could not be read or processed) in the *CommuniGate Pro* *BadFiles* directory for further analysis by the developer.

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
