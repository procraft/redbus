import {
  ActionIcon,
  AppShell,
  Burger,
  Button,
  Center,
  Container,
  Group,
  Loader,
  Stack,
  Text,
  ThemeIcon,
  Tooltip,
  useMantineColorScheme,
} from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { BusFront, Cable, Layers3, LayoutDashboard, Moon, Sun, TriangleAlert } from 'lucide-react';
import { lazy, Suspense } from 'react';
import { Navigate, Route, Routes, useLocation, useNavigate } from 'react-router';

const DashboardPage = lazy(() =>
  import('@/pages/DashboardPage').then((module) => ({ default: module.DashboardPage })),
);
const ConsumersPage = lazy(() =>
  import('@/pages/ConsumersPage').then((module) => ({ default: module.ConsumersPage })),
);
const FailedRepeatsPage = lazy(() =>
  import('@/pages/FailedRepeatsPage').then((module) => ({ default: module.FailedRepeatsPage })),
);
const TopicsPage = lazy(() =>
  import('@/pages/TopicsPage').then((module) => ({ default: module.TopicsPage })),
);

const navigation = [
  { label: 'Dashboard', path: '/', icon: LayoutDashboard },
  { label: 'Topics', path: '/topics', icon: Layers3 },
  { label: 'Consumers', path: '/consumers', icon: Cable },
  { label: 'Failed repeats', path: '/failed-repeats', icon: TriangleAlert },
] as const;

export default function App() {
  const [mobileNavigationOpened, { close, toggle }] = useDisclosure(false);
  const { colorScheme, toggleColorScheme } = useMantineColorScheme();
  const location = useLocation();
  const navigate = useNavigate();

  const navigateTo = (path: string) => {
    navigate(path);
    close();
  };

  const navigationButtons = navigation.map((item) => (
    <Button
      key={item.path}
      color="gray"
      justify="flex-start"
      leftSection={<item.icon size={17} />}
      onClick={() => navigateTo(item.path)}
      variant={location.pathname === item.path ? 'light' : 'subtle'}
    >
      {item.label}
    </Button>
  ));

  return (
    <AppShell
      header={{ height: 64 }}
      navbar={{
        width: 250,
        breakpoint: 'sm',
        collapsed: { desktop: true, mobile: !mobileNavigationOpened },
      }}
      padding={{ base: 'md', sm: 'lg' }}
    >
      <AppShell.Header>
        <Container size="xl" h="100%">
          <Group h="100%" justify="space-between" wrap="nowrap">
            <Group gap="sm" wrap="nowrap">
              <Burger
                aria-label="Toggle navigation"
                hiddenFrom="sm"
                opened={mobileNavigationOpened}
                onClick={toggle}
                size="sm"
              />
              <ThemeIcon color="red" size="lg" variant="filled">
                <BusFront size={22} />
              </ThemeIcon>
              <Text fw={800} visibleFrom="xs">
                RED Bus
              </Text>
            </Group>

            <Group gap={4} visibleFrom="sm">
              {navigationButtons}
            </Group>

            <Tooltip label={`Use ${colorScheme === 'dark' ? 'light' : 'dark'} theme`}>
              <ActionIcon
                aria-label="Toggle color scheme"
                onClick={toggleColorScheme}
                size="lg"
                variant="subtle"
              >
                {colorScheme === 'dark' ? <Sun size={19} /> : <Moon size={19} />}
              </ActionIcon>
            </Tooltip>
          </Group>
        </Container>
      </AppShell.Header>

      <AppShell.Navbar p="md">
        <Stack gap="xs">{navigationButtons}</Stack>
      </AppShell.Navbar>

      <AppShell.Main>
        <Container size="xl">
          <Suspense
            fallback={
              <Center py="xl">
                <Loader />
              </Center>
            }
          >
            <Routes>
              <Route path="/" element={<DashboardPage />} />
              <Route path="/topics" element={<TopicsPage />} />
              <Route path="/consumers" element={<ConsumersPage />} />
              <Route path="/failed-repeats" element={<FailedRepeatsPage />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </Suspense>
        </Container>
      </AppShell.Main>
    </AppShell>
  );
}
