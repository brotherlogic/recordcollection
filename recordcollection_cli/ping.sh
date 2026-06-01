recordfanout_cli ping --id $(recordcollection_cli get 482391 | head -n 1 | awk '{print $3}' | awk -F ':' '{print $2}')
