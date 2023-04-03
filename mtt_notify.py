import sqlite3
import http.client
import urllib
import os
import yaml
import argparse
import sys
import logging

from sqlite3 import Error
from datetime import datetime

#
# Templates
#
BASE_TEMPLATE = '''Hi,

%%ONGOING%%
%%DUE_DATE%%'''
ONGOING_TEMPLATE = '''=== Ongoing tasks: ===
%%ONGOING_TASK%%'''
DUEDATE_TEMPLATE = '''=== Due date: ===
%%DUEDATE_TASK%%'''
#
# end of Templates
#

__version__ = 0.4  # alpha
log = logging.getLogger('mtt_notify')

TAG_ID = 13  # "ongoing" tag
DUE_DATE_THRESHOLD_DAYS = 7


class Configuration:
    app_name = ''
    config_full_path = ''
    config = []

    def __init__(self, app_name):
        self.app_name = app_name

        config_known_paths = [
            f'{os.getcwd()}/{app_name}.yaml',
            f'/etc/{app_name}/{app_name}.yaml',
            f'/etc/{app_name}.yaml',
            f'/usr/local/etc/{app_name}/{app_name}.yaml',
            f'/usr/local/etc/{app_name}.yaml',
        ]

        for config_path in config_known_paths:
            if not os.path.exists(config_path):
                continue
            else:
                self.config_full_path = config_path
                break

        if not self.config_full_path:
            raise Exception('Configuration file not found')

        if not self._load_configuration():
            raise Exception('Configuration error')

    def _load_configuration(self):

        if len(self.config) > 0:
            return self.config

        file_handler = open(self.config_full_path, 'r')

        try:
            self.config = yaml.full_load(file_handler)
            # return self.config
            return True
        # except IOError:
        #     # log.error('Config file does not exist')
        #     return []
        except yaml.parser.ParserError:
            raise Exception('Config syntax error')

    def __getitem__(self, item):
        return self.config[item]


def create_connection(db_file):
    """ create a database connection to the SQLite database
        specified by db_file
    :param db_file: database file
    :return: Connection object or None
    """
    conn = None
    try:
        conn = sqlite3.connect(db_file)
    except Error as e:
        print(e)

    return conn


def main(args=None):
    if args is None:
        args = sys.argv[1:]

    log.setLevel(logging.INFO)
    fh = logging.StreamHandler(sys.stdout)
    fh.setFormatter(logging.Formatter('%(message)s'))
    log.addHandler(fh)

    parser = argparse.ArgumentParser(
        description='\033[93m[[ {} ]]\033[0m tool v{}'.format('myTinyTodo Notifier', __version__),
    )

    parser.add_argument('-d', '--debug', dest='debug', action='store_true', default=False,
                        help='causes the tool to print debugging messages about its progress')
    parser.add_argument('-s', '--silent', dest='silent', action='store_true', default=False,
                        help='do not output general info but errors, useful for crontab')
    parser.add_argument('--disable-notification', dest='disable_notify', action='store_true', default=False,
                        help='do not send actual notification, useful for debugging')

    args = parser.parse_args(args)

    if args.silent is True:
        log.setLevel(logging.CRITICAL)

    if args.debug is True:
        log.setLevel(logging.DEBUG)
        log.debug("Debug message mode is enabled")

    conf = Configuration('mtt_notify')
    # todo:
    #   conf.defaults({'api': 'api.pushover.net:443'})
    conn = create_connection(conf['database_path'])

    # todo:
    #     FIX THIS EXCEPTION! Config class should ask politely to provide missing config options
    #     Traceback (most recent call last):
    #     File "/home/stan/apps/mtt_notify.py", line 199, in <module>
    #       main()
    #     File "/home/stan/apps/mtt_notify.py", line 191, in main
    #       "token": conf['pushover_token'],
    #     File "/home/stan/apps/mtt_notify.py", line 82, in __getitem__
    #       return self.config[item]
    #   KeyError: 'pushover_token'
    #

    with conn:
        cursor = conn.cursor()

        log.debug('[>] Searching SQLITE for the DUE DATE tasks...')
        cursor.execute(f'''
            SELECT * FROM mtt_todolist
            WHERE compl = 0
            AND duedate IS NOT null
            AND (
                duedate > DATE('now', '-{DUE_DATE_THRESHOLD_DAYS} days')
                OR duedate <= DATE('now')
            )
        ''')
        rows = cursor.fetchall()

        log.info('[*] DUE DATE tasks:')
        tasks_duedate = []
        for row in rows:
            log.debug(f"ROW: {row}")  # print(row)
            # https://docs.python.org/3/library/datetime.html
            # datetime.strptime('2015-05-16', '%Y-%m-%d')
            dt_obj = datetime.strptime(row[13], '%Y-%m-%d')
            message = f"- \"{row[7]}\" {'EXPIRED' if dt_obj < datetime.now() else 'due'} on {dt_obj:%A, %B %e} ({dt_obj:%m/%d})"
            log.debug(f"LINE: \"{message}\"")
            tasks_duedate.append(message)

        log.debug('[>] Searching SQLITE for the tag: (Searching for the tag "ongoing")')
        cursor.execute('''
            SELECT * FROM mtt_tag2task
            INNER JOIN mtt_todolist
                ON mtt_todolist.id = mtt_tag2task.task_id
            WHERE mtt_tag2task.tag_id = ? and mtt_todolist.compl = 0
        ''', (TAG_ID,))

        rows = cursor.fetchall()
        task_tag = []

        log.info(f"[*] Tagged tasks:")
        for row in rows:
            log.info(f"Task \"{row[10]}\" has \"{row[14]}\" tag")
            task_tag.append(f"- {row[10]}")

        if not tasks_duedate and not task_tag:
            return True

        ONGOING_STR = ONGOING_TEMPLATE.replace('%%ONGOING_TASK%%', "\n".join(task_tag))
        DUEDATE_STR = DUEDATE_TEMPLATE.replace('%%DUEDATE_TASK%%', "\n".join(tasks_duedate))
        BASE_STR = BASE_TEMPLATE.replace('%%ONGOING%%', ONGOING_STR).replace('%%DUE_DATE%%', DUEDATE_STR)

        log.debug(f"[*] The notification below is about to send out:\n"
                  f"========== [START] ==========\n"
                  f"{BASE_STR}\n"
                  f"========== [END] ==========")

        if args.disable_notify is True:
            log.info("[!] Not notifying because instructed to do so")
        else:
            conn = http.client.HTTPSConnection("api.pushover.net:443")
            conn.request("POST", "/1/messages.json",
                         urllib.parse.urlencode({
                             "token": conf['pushover_token'],
                             "user": conf['pushover_user'],
                             "message": BASE_STR,
                         }), {"Content-type": "application/x-www-form-urlencoded"})
            conn.getresponse()


if __name__ == '__main__':
    main()
