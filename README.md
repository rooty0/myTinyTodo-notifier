# myTinyTodo-notifier
Notifier for myTinyTodo [(maxpozdeev/mytinytodo)](https://github.com/maxpozdeev/mytinytodo)

![Screen Shot](https://repository-images.githubusercontent.com/622862195/3bd4e263-c7fd-409d-a30d-f9bef9abf5a6)


Never miss your task deadline again!

This tool expands the capabilities of [myTinyTodo](https://www.mytinytodo.net/) by providing a notification feature based on the following:
- due date
- special tag

Please note, at the time, only one delivery method is supported:
- [Pushover](https://pushover.net/)



## Usage
Just run once in a awhile with crontab
```bash
$ crontab -l
# .---------------- minute (0 - 59)
# |     .------------- hour (0 - 23)
# |     |       .---------- day of month (1 - 31)
# |     |       |       .------- month (1 - 12) OR jan,feb,mar,apr ...
# |     |       |       |       .---- day of week (0 - 6) (Sunday=0 or 7)  OR sun,mon,tue,wed,thu,fri,sat
# |     |       |       |       |       commands
#
#
30 20 * * * /usr/local/bin/python3.9 /usr/local/bin/mtt_notify.py --silent
```

## Future Improvements
- see source code

## Contribute
Feel free to create a PR
