# go-toon

Eneco Toon client in Go. Very simple controls right now, but basics are working:

    ./go-toon -username USERNAME -password PASSWORD -temp
	Current temperature: 20.64
	Active state: comfort

Thanks to [rvdm/toon](https://github.com/rvdm/toon) for having figured out the "API".

## TODO

- turn go-toon into a proper library with a thin frontend
- add support for additional features:
	- retrieve power usage
	- retrieve current program state
- configuration file with credentials

## Copyright

2015 Jasper Lievisse Adriaanse <j@jasper.la> released under the MIT license.

## Contributing
1. Fork it!
2. Create your feature branch: `git checkout -b my-new-feature`
3. Commit your changes: `git commit -am 'Add some feature'`
4. Push to the branch: `git push origin my-new-feature`
5. Submit a pull request
