[![English](https://img.shields.io/badge/lang-en-lightgrey.svg)](README.md) [![Russian](https://img.shields.io/badge/lang-ru-red.svg)](#)

## DrWeb helper for CommuniGate Pro 6.x, *версия 2.1.0*

### Введение

*Помощник* (внешний фильтр сообщений) **drweb-cgp** используется для проверки почтовых сообщений на вирусы. Он получает сообщения от *CommuniGate Pro* и передаёт их для обработки в *DrWeb*.<br/><br/>

### Возможности

* *Помощник* получает сообщение от *CommuniGate Pro* по протоколу *Интерфейса Внешнего Фильтра* и передаёт сообщение в *DrWeb*, используя протокол *CLAMD*.

* *Помощник* взаимодействует с *DrWeb* используя UNIX-сокет или TCP. При использовании UNIX-сокета задействуется механизм передачи файловых дескрипторов (FILDES), это исключает копирование данных сообщения между процессами хелпера и DrWeb.

> Важно: При использовании UNIX-сокета убедитесь, что пользователь, от имени которого запущен DrWeb, имеет права на чтение файлов в очереди CommuniGate Pro.

* *Помощник* даёт команду *CommuniGate Pro* удалить сообщение, если в нём обнаружен вирус.

* *Помощник* может выполнять пересылку копии сообщения с вирусом на указанные почтовые адреса.

* Если выполняется пересылка копии сообщения с вирусом, чтобы избежать двойной обработки или зацикливания, *Помощник* добавляет в обработанное сообщение заголовок *X-DrWeb-Seen:*.

### Установка

> Ниже показаны настройки *Помощника* и *Правила* в интерфейсе *CommuniGate Pro*.

##### Установки -> Общее -> Помощники
![](./helper.png)

##### Установки -> Почта -> Правила -> DRWEB
![](./rule.png)


#### Параметры командной строки

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

В режиме -debug хелпер сохраняет копии проблемных файлов (которые не удалось прочитать или обработать) в директории *CommuniGate Pro* *BadFiles* для последующего анализа разработчиком.

**host**

Путь до UNIX-сокета или сетевой адрес (IP или имя хоста). Если порт не указан, используется *3310*.

**mirror-to**

Список почтовых адресов (через запятую), куда будут пересылаться копии сообщений с вирусами.

**timeout**

Тайм-аут обработки одного сообщения в *DrWeb*.

</br>

### Лицензия

BSD License, [LICENSE.md](LICENSE.md)<br/><br/>

### Автор

Andrey Igoshin <<ai@vsu.ru>><br/><br/>

### Ссылки

* Репозиторий: <https://git.vsu.ru/ai/drweb-cgp>
* Сайт CommuniGate Pro: <https://communigatepro.ru>
* Протокол Помощника: <https://doc.communigatepro.ru/russian/development/Helpers.html#Filters>
* Сайт DrWeb: <https://www.drweb.ru/>
