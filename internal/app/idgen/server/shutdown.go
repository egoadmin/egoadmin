package server

func configureShutdown(opts Options) {
	if opts.shutdown == nil {
		return
	}
	opts.shutdown.RegisterRegistry(opts.registry)
	opts.shutdown.RegisterDB("mysql", opts.db)
	opts.shutdown.Bind(opts.app)
}
