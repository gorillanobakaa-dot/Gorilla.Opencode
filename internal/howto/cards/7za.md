# 7za (embedded toolbox)

7-Zip 26.04, the same name on Windows and Linux. Packs and unpacks .7z, .zip, .tar, .gz, .xz and more. `x` keeps
the folders inside the archive, `e` flattens them; `-o` takes the target folder with NO space after it. `-y` answers
yes to every overwrite: use it only on a new, empty target.

    7za l backup.7z                                  # list what is inside, changes nothing
    7za t backup.7z                                  # test the archive (checksums), changes nothing
    7za x backup.7z -oout                            # unpack into ./out with its folders
    7za a -t7z -mx=9 backup.7z src docs              # pack two folders, best compression
    7za a -tzip site.zip public/*                     # a .zip for someone without 7-Zip
