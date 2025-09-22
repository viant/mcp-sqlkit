package connector

import "context"

// Remove deletes a connector from the caller's namespace.
func (s *Service) Remove(ctx context.Context, name string) {
	namespace, err := s.auth.Namespace(ctx)
	if err != nil {
		return
	}
	ns, ok := s.namespace.Get(namespace)
	if !ok {
		return
	}
	ns.Connectors.Delete(name)
}
