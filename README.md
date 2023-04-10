# myTinyTodo-notifier
notifier for myTinyTodo (maxpozdeev/mytinytodo)

## Usage
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
