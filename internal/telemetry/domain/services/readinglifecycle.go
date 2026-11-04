package services

import "context"

type ReadingLifecycle interface{ Enter(context.Context) func() }
