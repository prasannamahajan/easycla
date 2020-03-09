// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

import { Component, ViewChild } from '@angular/core';
import { App, Nav, Platform } from 'ionic-angular';
import { StatusBar } from '@ionic-native/status-bar';
import { SplashScreen } from '@ionic-native/splash-screen';

import { CincoService } from '../services/cinco.service';
import { KeycloakService } from '../services/keycloak/keycloak.service';
import { RolesService } from '../services/roles.service';
import { ClaService } from '../services/cla.service';
import { HttpClient } from '../services/http-client';

import { AuthService } from '../services/auth.service';
import { AuthPage } from '../pages/auth/auth';
import { EnvConfig } from '../services/cla.env.utils';

@Component({
  templateUrl: 'app.html'
})
export class MyApp {
  @ViewChild(Nav) nav: Nav;

  rootPage: any = AuthPage;

  userRoles: any;
  pages: Array<{
    icon?: string;
    access: boolean;
    title: string;
    component: any;
  }>;

  users: any[];

  constructor(
    public platform: Platform,
    public app: App,
    public statusBar: StatusBar,
    public splashScreen: SplashScreen,
    private cincoService: CincoService,
    private keycloak: KeycloakService,
    private rolesService: RolesService,
    public claService: ClaService,
    public httpClient: HttpClient,
    public authService: AuthService
  ) {
    this.getDefaults();
    this.initializeApp();

    // Determine if we're running in a local services (developer) mode - the USE_LOCAL_SERVICES environment variable
    // will be set to true, otherwise were using normal services deployed in each environment
    const localServicesMode = (process.env.USE_LOCAL_SERVICES || 'false').toLowerCase() === 'true';
    // Set true for local debugging using localhost (local ports set in claService)
    this.claService.isLocalTesting(localServicesMode);

    this.claService.setApiUrl(EnvConfig['cla-api-url']);
    this.claService.setHttp(httpClient);
    // here to check authentication state
    this.authService.handleAuthentication();
  }

  getDefaults() {
    this.pages = [];
    this.userRoles = this.rolesService.userRoles;
    this.regeneratePagesMenu();
  }

  ngOnInit() {
    this.rolesService.getUserRolesPromise().then((userRoles) => {
      this.userRoles = userRoles;
      this.regeneratePagesMenu();
    });
  }

  initializeApp() {
    this.platform.ready().then(() => {
      // Okay, so the platform is ready and our plugins are available.
      // Here you can do any higher level native things you might need.
      // this.statusBar.styleDefault();
      // this.splashScreen.hide();
    });
  }

  openPage(page) {
    // Set the nav root so back button doesn't show
    this.nav.setRoot(page.component);
  }

  regeneratePagesMenu() {
    this.pages = [
      {
        title: 'All Companies',
        access: true,
        component: 'CompaniesPage'
      },
      {
        title: 'Sign Out',
        access: true,
        component: 'LogoutPage'
      }
    ];
  }
}
